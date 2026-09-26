package repositories

import (
	"context"
	"errors"

	geoModels "josex/web/modules/geo/models"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The GPS slice of TRACK-010: the one writer of positions, the driver's vehicle, the partitions.

// IngestPositions stores a batch — points is sp_ingest_positions' JSON array — and answers one row per
// vehicle of it.
func (r *TrackingRepository) IngestPositions(ctx context.Context, tenantID uuid.UUID, points []byte, arrivalRadiusM int) ([]models.IngestedVehicle, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT vehicle_id, trip_id, last, rider_ids, organization_ids, arrived_stop_id
		 FROM tracking.sp_ingest_positions(p_tenant_id := $1, p_points := $2::JSONB, p_arrival_radius_m := $3)`,
		tenantID, points, arrivalRadiusM)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.IngestedVehicle, error) {
		var v models.IngestedVehicle
		err := row.Scan(&v.VehicleID, &v.TripID, &v.Last, &v.RiderIDs, &v.OrganizationIDs, &v.ArrivedStopID)
		return v, err
	})
}

// DriverActiveVehicle answers the vehicle of the driver account's trip in progress, nil when there is
// none.
func (r *TrackingRepository) DriverActiveVehicle(ctx context.Context, tenantID, userID uuid.UUID) (*uuid.UUID, error) {
	var vehicleID, tripID uuid.UUID
	err := r.dbService.QueryRow(ctx,
		`SELECT vehicle_id, trip_id FROM tracking.sp_driver_active_vehicle(p_tenant_id := $1, p_user_id := $2)`,
		tenantID, userID).Scan(&vehicleID, &tripID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &vehicleID, nil
}

// PositionsPartitions creates aheadDays of day partitions and drops the ones past retentionDays.
func (r *TrackingRepository) PositionsPartitions(ctx context.Context, aheadDays, retentionDays int) (created, dropped int, err error) {
	err = r.dbService.QueryRow(ctx,
		`SELECT created, dropped FROM tracking.sp_positions_partitions(p_ahead_days := $1, p_retention_days := $2)`,
		aheadDays, retentionDays).Scan(&created, &dropped)
	return created, dropped, err
}

// TripTracePoints answers the trip's GPS points, oldest first.
func (r *TrackingRepository) TripTracePoints(ctx context.Context, tenantID, tripID uuid.UUID) ([]geoModels.LatLng, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT lat, lng FROM tracking.sp_trip_trace_points(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (geoModels.LatLng, error) {
		var p geoModels.LatLng
		err := row.Scan(&p.Lat, &p.Lng)
		return p, err
	})
}

// CloseTripTrace stores the trip's driven path, its km and where the line came from.
func (r *TrackingRepository) CloseTripTrace(ctx context.Context, tenantID, tripID uuid.UUID, polyline6 string, km float64, source string) error {
	_, err := r.dbService.Execute(ctx,
		`SELECT tracking.sp_trip_close_trace(p_tenant_id := $1, p_trip_id := $2, p_polyline := $3, p_distance_km := $4, p_source := $5)`,
		tenantID, tripID, polyline6, km, source)
	return err
}
