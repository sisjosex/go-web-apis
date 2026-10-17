package repositories

import (
	"context"
	"errors"
	"time"

	coreServices "josex/web/modules/core/services"
	geoModels "josex/web/modules/geo/models"
	"josex/web/modules/tracking/interfaces"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// HistoricalDurations is geo's second ETA source (TRACK-066 D2): the median of the seconds observed
// between the stops nearest two points. It reads the tenant the caller put in the context with
// interfaces.WithTenant; none, or no stops or too few samples there, is ok=false.
type HistoricalDurations struct {
	dbService coreServices.DatabaseService
}

func NewHistoricalDurations(dbService coreServices.DatabaseService) *HistoricalDurations {
	return &HistoricalDurations{dbService: dbService}
}

func (h *HistoricalDurations) Duration(ctx context.Context, from, to geoModels.LatLng, _ string) (float64, bool, error) {
	tenantID, ok := interfaces.TenantFrom(ctx)
	if !ok {
		return 0, false, nil
	}
	var seconds int
	err := h.dbService.QueryRow(ctx,
		`SELECT seconds FROM tracking.fn_stop_pair_duration(p_tenant_id := $1, p_from_lat := $2, p_from_lng := $3,
			p_to_lat := $4, p_to_lng := $5, p_at := $6)`,
		tenantID, from.Lat, from.Lng, to.Lat, to.Lng, time.Now()).Scan(&seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return float64(seconds), true, nil
}

// RecordStopPairSamples stores a completed trip's stop-to-stop seconds (TRACK-066 D1); a replay adds none.
func (r *TrackingRepository) RecordStopPairSamples(ctx context.Context, tenantID, tripID uuid.UUID) (int, error) {
	var added int
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_record_stop_pair_samples(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID).Scan(&added)
	return added, err
}

// PruneStopPairSamples drops the tenant's samples older than days.
func (r *TrackingRepository) PruneStopPairSamples(ctx context.Context, tenantID uuid.UUID, days int) (int, error) {
	var deleted int
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_stop_pair_samples_prune(p_tenant_id := $1, p_days := $2)`, tenantID, days).Scan(&deleted)
	return deleted, err
}
