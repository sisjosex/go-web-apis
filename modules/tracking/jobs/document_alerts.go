// Package jobs is the tracking module's background work (INFRA-001 D4): the daily document-alerts pass
// the scheduler fires, and the outbox handler that mails what a pass claimed.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskDocumentAlerts runs the pass over every tenant; the scheduler fires it daily.
	TaskDocumentAlerts = "tracking:document-alerts"
	// DocumentAlertsCron is 06:00 UTC: the digest is in the inbox before the working day starts.
	DocumentAlertsCron = "0 6 * * *"
	// topicDocumentAlerts is what sp_raise_document_alerts writes to tracking.outbox.
	topicDocumentAlerts = "document.alerts"
)

// TaskDocumentDigest is the outbox task a pass's row becomes.
var TaskDocumentDigest = coreJobs.OutboxTask(topicDocumentAlerts)

// PassResult is one tenant's share of a pass.
type PassResult struct {
	Tenant  coreJobs.Tenant
	Claimed int
	Err     error
}

// RunDocumentAlertsPass claims every tenant's due documents. The SP writes the outbox row in the same
// statement, so this pass sends nothing itself: the mail leaves from the worker once the row commits.
// A tenant that fails is reported and the walk continues — one unreachable database must not cost
// every other tenant its digest.
func RunDocumentAlertsPass(ctx context.Context, db coreServices.DatabaseService, tenants []coreJobs.Tenant) []PassResult {
	repo := trackingRepos.NewTrackingRepository(db)
	results := make([]PassResult, 0, len(tenants))
	for _, tenant := range tenants {
		result := PassResult{Tenant: tenant}
		tenantID, err := uuid.Parse(tenant.ID)
		if err == nil {
			// The same key the tenant middleware sets on a request: every query in this context runs
			// against the tenant's own pool.
			tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
			claimed, raiseErr := repo.RaiseDocumentAlerts(tenantCtx, tenantID)
			result.Claimed, err = len(claimed), raiseErr
		}
		result.Err = err
		results = append(results, result)
	}
	return results
}

// PassHandler is the TaskDocumentAlerts handler: the pass over whatever tenants exist when it fires.
func PassHandler(db coreServices.DatabaseService, tenants coreJobs.TenantLister) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants(ctx)
		if err != nil {
			return err
		}
		failed := 0
		for _, r := range RunDocumentAlertsPass(ctx, db, list) {
			if r.Err != nil {
				log.Printf("⚠️  document alerts %s: %v", r.Tenant.ID, r.Err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("document alerts failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}

// TenantContact is who a tenant's digest goes to, and the name it is addressed in.
type TenantContact struct {
	Name   string
	Emails []string
}

// TenantDirectory looks a tenant up in the platform database. cmd/server builds it from the tenancy
// module, which tracking does not import.
type TenantDirectory func(ctx context.Context, tenantID uuid.UUID) (*TenantContact, error)

// digestDocument is one document as the SP wrote it into the outbox payload.
type digestDocument struct {
	SubjectName *string             `json:"subject_name"`
	TypeName    string              `json:"type_name"`
	Number      *string             `json:"number"`
	ExpiresOn   coreModels.DateOnly `json:"expires_on"`
	DaysLeft    int32               `json:"days_left"`
}

type digestPayload struct {
	TenantID  uuid.UUID        `json:"tenant_id"`
	Documents []digestDocument `json:"documents"`
}

// DigestHandler is the outbox:document.alerts handler: it mails the documents one pass claimed to the
// tenant's owners and admins. The payload carries everything the mail says, so nothing is read back
// from the tenant's database.
func DigestHandler(email coreServices.EmailService, directory TenantDirectory) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload digestPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			// A malformed row will not parse on a retry either.
			return fmt.Errorf("document digest payload: %v: %w", err, asynq.SkipRetry)
		}
		contact, err := directory(ctx, payload.TenantID)
		if err != nil {
			return err
		}
		if contact == nil || len(contact.Emails) == 0 {
			// A tenant with no owner is a misconfiguration to fix, not a reason to send later.
			log.Printf("⚠️  document digest %s: no owner or admin to mail", payload.TenantID)
			return nil
		}
		return sendDocumentDigest(email, contact, payload.Documents)
	}
}

// sendDocumentDigest mails one digest per recipient. It fails — and asynq retries — only when nobody
// got it: a retry after a partial send would mail the recipients who already have it a second time.
func sendDocumentDigest(email coreServices.EmailService, contact *TenantContact, documents []digestDocument) error {
	appConf := config.ModularAppConfig.Core
	subject := fmt.Sprintf("%d document(s) need attention in %s", len(documents), contact.Name)
	templatePath := coreServices.GetTemplatePath("tracking", "document-expiry-digest.html")

	data := map[string]interface{}{
		"Title":         subject,
		"TenantName":    contact.Name,
		"Description":   "These compliance documents have entered their warning window. A document is named here once; renew it and it will be watched again.",
		"Documents":     digestLines(documents),
		"ButtonText":    "Open documents",
		"DocumentsURL":  fmt.Sprintf("%s/tracking/documents", appConf.FrontendURL),
		"SignOff":       fmt.Sprintf("The %s team", appConf.AppName),
		"AutomatedNote": "This is an automated message. Please do not reply.",
	}

	sent := 0
	var lastErr error
	for _, to := range contact.Emails {
		if err := email.SendEmail(to, subject, templatePath, data); err != nil {
			log.Printf("⚠️  document digest %s: could not mail %s: %v", contact.Name, to, err)
			lastErr = err
			continue
		}
		sent++
	}
	if sent == 0 {
		return errors.Join(errors.New("document digest: no recipient could be mailed"), lastErr)
	}
	log.Printf("📧 document digest %s: %d document(s) mailed to %d recipient(s)", contact.Name, len(documents), sent)
	return nil
}

// digestLines turns the payload into what the template renders, so the template holds no logic.
func digestLines(documents []digestDocument) []map[string]string {
	lines := make([]map[string]string, 0, len(documents))
	for _, doc := range documents {
		subject := "—"
		if doc.SubjectName != nil {
			subject = *doc.SubjectName
		}
		number := ""
		if doc.Number != nil {
			number = *doc.Number
		}
		lines = append(lines, map[string]string{
			"Subject":   subject,
			"Type":      doc.TypeName,
			"Number":    number,
			"ExpiresOn": time.Time(doc.ExpiresOn).Format(time.DateOnly),
			"When":      expiryPhrase(doc.DaysLeft),
		})
	}
	return lines
}

// expiryPhrase says what the number of days means, so the reader does not do the arithmetic.
func expiryPhrase(daysLeft int32) string {
	switch {
	case daysLeft < 0:
		return fmt.Sprintf("expired %d day(s) ago", -daysLeft)
	case daysLeft == 0:
		return "expires today"
	default:
		return fmt.Sprintf("expires in %d day(s)", daysLeft)
	}
}
