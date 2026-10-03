//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	"josex/web/modules/core/testhelpers"
	coreUtils "josex/web/modules/core/utils"
	trackingJobs "josex/web/modules/tracking/jobs"
)

// capturedMail records what the digest handler would have sent.
type capturedMail struct {
	to, subject string
}

type mailRecorder struct {
	sent []capturedMail
}

func (m *mailRecorder) SendEmail(to, subject, _ string, _ interface{}) error {
	m.sent = append(m.sent, capturedMail{to, subject})
	return nil
}

func (m *mailRecorder) SendPlainEmail(to, subject, _ string) error {
	m.sent = append(m.sent, capturedMail{to, subject})
	return nil
}

func documentAlertRows(t *testing.T, helper *testhelpers.ApiTestHelper) []string {
	t.Helper()
	rows, err := helper.DB().Query(context.Background(),
		`SELECT payload::text FROM tracking.outbox WHERE topic = 'document.alerts' ORDER BY id`)
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()
	payloads := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		payloads = append(payloads, p)
	}
	return payloads
}

// TestDocumentAlertsPass_OneRowThenNone - D4: a pass that claims a due document writes exactly one
// outbox row carrying it, a second pass writes nothing, and the row's payload is all the digest
// handler needs to mail the tenant's admins.
func TestDocumentAlertsPass_OneRowThenNone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	ctx := context.Background()
	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	due := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 10)
	tenants := []coreJobs.Tenant{{ID: TestTenantID, DatabaseURL: config.ModularAppConfig.Core.DatabaseURL}}
	before := len(documentAlertRows(t, helper))

	first := trackingJobs.RunDocumentAlertsPass(ctx, helper.DB(), tenants)
	afterFirst := documentAlertRows(t, helper)
	second := trackingJobs.RunDocumentAlertsPass(ctx, helper.DB(), tenants)
	afterSecond := documentAlertRows(t, helper)

	if first[0].Err != nil || first[0].Claimed == 0 {
		t.Fatalf("first pass: claimed %d, err %v", first[0].Claimed, first[0].Err)
	}
	if len(afterFirst) != before+1 {
		t.Fatalf("first pass wrote %d outbox rows, want 1", len(afterFirst)-before)
	}
	row := afterFirst[len(afterFirst)-1]
	if !strings.Contains(row, due["id"].(string)) || !strings.Contains(row, TestTenantID) {
		t.Fatalf("outbox payload lacks the document or the tenant: %s", row)
	}
	if second[0].Err != nil || second[0].Claimed != 0 || len(afterSecond) != len(afterFirst) {
		t.Fatalf("second pass: claimed %d, %d new rows, err %v — want nothing", second[0].Claimed, len(afterSecond)-len(afterFirst), second[0].Err)
	}

	loadTrackingTranslations(t)
	mail := &mailRecorder{}
	directory := func(_ context.Context, id uuid.UUID) (*trackingJobs.TenantContact, error) {
		return &trackingJobs.TenantContact{Name: "Test tenant " + id.String()[:4], Recipients: []trackingJobs.Recipient{{Email: "owner@test.local", Locale: "es"}, {Email: "admin@test.local", Locale: "en"}}}, nil
	}
	task := asynq.NewTask(trackingJobs.TaskDocumentDigest, []byte(row))
	if err := trackingJobs.DigestHandler(mail, directory)(ctx, task); err != nil {
		t.Fatalf("digest handler: %v", err)
	}
	// Each admin in the language they read (APP-009 D2): the owner Spanish, the admin English.
	if len(mail.sent) != 2 || !strings.Contains(mail.sent[0].subject, "requieren atención") ||
		!strings.Contains(mail.sent[1].subject, "need attention") {
		t.Fatalf("digest sent %+v, want one mail to each of the two admins, each in their language", mail.sent)
	}
}

// loadTrackingTranslations puts this module's lang files where the email code reads them; the server
// loads every module's at start, a test runs from its own directory.
func loadTrackingTranslations(t *testing.T) {
	t.Helper()
	for _, lang := range []string{"en", "es"} {
		raw, err := os.ReadFile(filepath.Join("..", "lang", lang+".json"))
		if err != nil {
			t.Fatalf("read %s translations: %v", lang, err)
		}
		values := map[string]string{}
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s translations: %v", lang, err)
		}
		if coreUtils.Translations[lang] == nil {
			coreUtils.Translations[lang] = map[string]string{}
		}
		for key, value := range values {
			coreUtils.Translations[lang][key] = value
		}
	}
}
