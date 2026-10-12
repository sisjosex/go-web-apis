package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// The gauges no exporter can know, read by docker/prometheus/alerts.yml (INFRA-018). Each is set where
// the process already looks — a probe, a drain, a read — so a scrape costs nothing but the read of
// memory; a family with no sample yet is simply absent from /metrics.
var (
	// DependencyUp is 1 while a dependency answers, 0 once it does not: Valkey from /readyz's ping
	// (every 15 s, the healthcheck), Valhalla from its breaker (open → 0, closed → 1).
	DependencyUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "taypi_dependency_up",
		Help: "Whether a dependency answered its last probe (1) or not (0).",
	}, []string{"dependency"})

	// OutboxUnpublishedRows and OutboxOldestUnpublishedSeconds are the relay's backlog per database,
	// read after each drain — on a NOTIFY or the poll tick — and after a failed one.
	OutboxUnpublishedRows = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "taypi_outbox_unpublished_rows",
		Help: "tracking.outbox rows not yet published as tasks, per database.",
	}, []string{"database"})
	OutboxOldestUnpublishedSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "taypi_outbox_oldest_unpublished_seconds",
		Help: "Age of the oldest unpublished tracking.outbox row, per database; 0 when none is waiting.",
	}, []string{"database"})

	// GPSStreamLagSeconds is how far the GPS consumer is behind each stream: the age of the oldest
	// entry not yet stored, 0 when the stream is drained.
	GPSStreamLagSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "taypi_gps_stream_lag_seconds",
		Help: "Age of the oldest GPS stream entry not yet stored, per stream; 0 when drained.",
	}, []string{"stream"})
)

// SetUp sets a 0/1 gauge from a bool.
func SetUp(g prometheus.Gauge, up bool) {
	if up {
		g.Set(1)
		return
	}
	g.Set(0)
}
