//go:build integration
// +build integration

package core_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize test environment (load .env.test before any config)
	testhelpers.InitTestEnvironment()
}

// The probes must answer without a token and without X-Tenant-Slug — an
// orchestrator has neither. DoRootRequest sends neither header.

func TestLivez_NoAuthRequired(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	w := helper.DoRootRequest(http.MethodGet, "/livez")

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("livez returned a non-JSON body: %s", w.Body.String())
	}
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz_OkWhenDatabaseAnswers(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	w := helper.DoRootRequest(http.MethodGet, "/readyz")

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("readyz returned a non-JSON body: %s", w.Body.String())
	}
	assert.Equal(t, "ok", body["status"])
}
