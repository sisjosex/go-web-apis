//go:build integration
// +build integration

package core_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreMiddleware "josex/web/modules/core/middleware"
	"josex/web/modules/core/testhelpers"

	"github.com/gin-gonic/gin"
)

// INFRA-011 D3: sign-in per IP and address, imports per tenant, on top of the per-IP limit.

func classEngine(t *testing.T) *gin.Engine {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	t.Cleanup(helper.Close)
	if helper.Valkey() == nil {
		t.Fatal("REDIS_URL is not set in .env.test — the rate class test needs Valkey (make docker-up)")
	}
	if err := helper.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	engine := gin.New()
	engine.Use(coreMiddleware.RateClasses(helper.Valkey(), 5, 3))
	engine.POST("/api/v1/auth/login", func(c *gin.Context) {
		// The controller still reads the body the limiter looked at.
		raw, _ := io.ReadAll(c.Request.Body)
		if !strings.Contains(string(raw), "email") {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusOK)
	})
	engine.POST("/api/v1/import", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine
}

func post(engine *gin.Engine, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestRateClasses_LoginPerIPAndEmail - the sixth sign-in a minute for one address is 429 with
// Retry-After; another address from the same IP still gets through.
func TestRateClasses_LoginPerIPAndEmail(t *testing.T) {
	engine := classEngine(t)
	for i := range 5 {
		if rec := post(engine, "/api/v1/auth/login", `{"email":"a@test.com"}`, nil); rec.Code != http.StatusOK {
			t.Fatalf("sign-in %d: want 200, got %d", i+1, rec.Code)
		}
	}
	rec := post(engine, "/api/v1/auth/login", `{"email":"A@test.com "}`, nil)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("sixth sign-in: want 429 with Retry-After, got %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := post(engine, "/api/v1/auth/login", `{"email":"b@test.com"}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("another address: want 200, got %d", rec.Code)
	}
}

// TestRateClasses_ImportPerTenant - the fourth import a minute of one tenant is 429; another tenant's
// import is not held back by it.
func TestRateClasses_ImportPerTenant(t *testing.T) {
	engine := classEngine(t)
	busy := map[string]string{"X-Tenant-Slug": "busy"}
	for i := range 3 {
		if rec := post(engine, "/api/v1/import", "{}", busy); rec.Code != http.StatusOK {
			t.Fatalf("import %d: want 200, got %d", i+1, rec.Code)
		}
	}
	if rec := post(engine, "/api/v1/import", "{}", busy); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("fourth import: want 429, got %d", rec.Code)
	}
	if rec := post(engine, "/api/v1/import", "{}", map[string]string{"X-Tenant-Slug": "calm"}); rec.Code != http.StatusOK {
		t.Fatalf("another tenant: want 200, got %d", rec.Code)
	}
}
