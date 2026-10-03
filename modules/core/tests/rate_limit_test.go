//go:build integration
// +build integration

package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coreMiddleware "josex/web/modules/core/middleware"
	"josex/web/modules/core/testhelpers"

	"github.com/gin-gonic/gin"
)

// rateLimitedEngine answers 200 behind RateLimit(10/s, burst 20), trusting X-Forwarded-For only from
// 10.0.0.0/8 — the shape production runs behind Caddy (INFRA-010).
func rateLimitedEngine(t *testing.T) *gin.Engine {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	t.Cleanup(helper.Close)
	if helper.Valkey() == nil {
		t.Fatal("REDIS_URL is not set in .env.test — the rate limit test needs Valkey (make docker-up)")
	}
	if err := helper.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatal(err)
	}
	engine.Use(coreMiddleware.RateLimit(helper.Valkey(), 10, 20))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine
}

func hit(engine *gin.Engine, remote, forwardedFor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	req.Header.Set("X-Forwarded-For", forwardedFor)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// TestRateLimitPerForwardedClient - behind a trusted proxy each X-Forwarded-For client has its own
// burst of 20; the 21st from one client is refused with Retry-After.
func TestRateLimitPerForwardedClient(t *testing.T) {
	engine := rateLimitedEngine(t)

	for i := range 20 {
		for _, client := range []string{"203.0.113.1", "203.0.113.2"} {
			if rec := hit(engine, "10.0.0.5:4000", client); rec.Code != http.StatusOK {
				t.Fatalf("request %d from %s: want 200, got %d", i+1, client, rec.Code)
			}
		}
	}
	rec := hit(engine, "10.0.0.5:4000", "203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("21st request: want 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 without Retry-After")
	}
}

// TestRateLimitIgnoresUntrustedForwardedFor - from an untrusted remote the header is ignored, so a
// client cannot pick a fresh key per request: all its requests share one bucket.
func TestRateLimitIgnoresUntrustedForwardedFor(t *testing.T) {
	engine := rateLimitedEngine(t)

	for i := range 20 {
		if rec := hit(engine, "198.51.100.7:4000", "203.0.113.1"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, rec.Code)
		}
	}
	if rec := hit(engine, "198.51.100.7:4000", "203.0.113.99"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("forged header: want 429 on the shared key, got %d", rec.Code)
	}
}
