package services

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// StartDiagnostics serves what never goes through Caddy (INFRA-011): GET /metrics for Prometheus on
// metricsAddr (the compose network), and the Go profiler on pprofAddr (localhost only, reached over SSH).
// An empty address turns its listener off; both stop with ctx.
func StartDiagnostics(ctx context.Context, metricsAddr, pprofAddr string) {
	if metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		serveDiagnostics(ctx, "metrics", metricsAddr, mux)
	}
	if pprofAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		serveDiagnostics(ctx, "pprof", pprofAddr, mux)
	}
}

func serveDiagnostics(ctx context.Context, name, addr string, handler http.Handler) {
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("⚠️  %s listener on %s: %v", name, addr, err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("📈 %s on %s", name, addr)
}
