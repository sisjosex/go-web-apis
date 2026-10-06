package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// requestDuration is what the Grafana dashboard reads for p50/p95 per route (INFRA-011 D1): the gin route
// pattern, never the raw path, so a million ids stay one series.
var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "API request latency by route pattern, method and status class.",
	Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
}, []string{"route", "method", "status"})

// Observe times every request: one histogram sample each, and one structured line for a request slower
// than slow or answered 5xx — route, tenant, status, duration_ms — the line a slow endpoint is found by
// (INFRA-011). The cost is a clock read and a histogram observe; nothing is logged for a fast 2xx–4xx.
func Observe(slow time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		elapsed := time.Since(start)

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		requestDuration.WithLabelValues(route, c.Request.Method, strconv.Itoa(status/100)+"xx").Observe(elapsed.Seconds())

		if elapsed >= slow || status >= 500 {
			slog.Warn("slow or failed request",
				"route", route,
				"method", c.Request.Method,
				"status", status,
				"duration_ms", elapsed.Milliseconds(),
				"tenant", c.GetString("tenant_slug"),
			)
		}
	}
}
