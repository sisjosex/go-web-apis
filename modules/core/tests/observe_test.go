//go:build integration
// +build integration

package core_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coreMiddleware "josex/web/modules/core/middleware"

	"github.com/gin-gonic/gin"
)

// INFRA-011: a request slower than the threshold writes one JSON line with route and duration_ms; a
// fast one writes nothing.
func TestObserve_SlowRequestLogsOneLine(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	engine := gin.New()
	engine.Use(coreMiddleware.Observe(100 * time.Millisecond))
	engine.GET("/slow/:id", func(c *gin.Context) { time.Sleep(150 * time.Millisecond); c.Status(http.StatusOK) })
	engine.GET("/fast", func(c *gin.Context) { c.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fast", nil))
	if out.Len() != 0 {
		t.Fatalf("a fast request logged: %s", out.String())
	}

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow/42", nil))
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one line, got %d: %s", len(lines), out.String())
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if line["route"] != "/slow/:id" {
		t.Fatalf("route: want the pattern, got %v", line["route"])
	}
	if ms, ok := line["duration_ms"].(float64); !ok || ms < 150 {
		t.Fatalf("duration_ms: %v", line["duration_ms"])
	}
}
