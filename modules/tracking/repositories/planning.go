package repositories

import (
	"context"
	"encoding/json"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The planning slice of TRACK-014: the overview reads, the run's lifecycle and the apply.

var optimizationCodes = map[string]string{
	"organization.not-found":               trackingErrors.OrganizationNotFound,
	"route.destination-required":           trackingErrors.RouteDestinationRequired,
	"vehicle.not-found":                    trackingErrors.VehicleNotFound,
	"optimization.not-found":               trackingErrors.OptimizationNotFound,
	"optimization.no-riders":               trackingErrors.OptimizationNoRiders,
	"optimization.no-vehicles":             trackingErrors.OptimizationNoVehicles,
	"optimization.applied":                 trackingErrors.OptimizationApplied,
	"optimization.not-ready":               trackingErrors.OptimizationNotReady,
	"optimization.vehicle-repeated":        trackingErrors.OptimizationVehicleRepeated,
	"optimization.stale":                   trackingErrors.OptimizationStale,
	trackingErrors.OrganizationScopeDenied: trackingErrors.OrganizationScopeDenied,
}

// mapOptimizationError maps an SP refusal to its code; a stale apply names the rider as its Detail.
func mapOptimizationError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := optimizationCodes[pgErr.Message]; ok {
			mapped := &trackingErrors.TrackingError{Code: code, Err: pgErr}
			if pgErr.Detail != "" {
				mapped.Detail = map[string]string{"rider_id": pgErr.Detail}
			}
			return mapped
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// PlanningDefaults answers the hours the screen opens on: the organization's running arrival and
// departure, else 07:45 and 13:00.
func (r *TrackingRepository) PlanningDefaults(ctx context.Context, tenantID, organizationID uuid.UUID) (string, string, error) {
	var arrival, departure string
	err := r.dbService.QueryRow(ctx,
		`SELECT arrival_time, departure_time FROM tracking.sp_planning_defaults(p_tenant_id := $1, p_organization_id := $2)`,
		tenantID, organizationID).Scan(&arrival, &departure)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: err}
	}
	if err != nil {
		return "", "", mapOptimizationError(err, trackingErrors.PlanningFailed)
	}
	return arrival, departure, nil
}

// PlanningRiders answers the organization's active riders for the planning map.
func (r *TrackingRepository) PlanningRiders(ctx context.Context, tenantID, organizationID uuid.UUID, days int16) ([]models.PlanningRider, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_planning_overview(p_tenant_id := $1, p_organization_id := $2, p_days := $3::SMALLINT)`,
		tenantID, organizationID, days)
	if err != nil {
		return nil, mapOptimizationError(err, trackingErrors.PlanningFailed)
	}
	defer rows.Close()
	riders := []models.PlanningRider{}
	for rows.Next() {
		var p models.PlanningRider
		if err := rows.Scan(&p.ID, &p.Name, &p.Lat, &p.Lng, &p.ServiceLegs,
			&p.OutboundRouteID, &p.OutboundRouteName, &p.ReturnRouteID, &p.ReturnRouteName); err != nil {
			return nil, err
		}
		riders = append(riders, p)
	}
	if err := rows.Err(); err != nil {
		return nil, mapOptimizationError(err, trackingErrors.PlanningFailed)
	}
	return riders, nil
}

// FleetAvailability answers every active vehicle, free or busy at the planned hours.
func (r *TrackingRepository) FleetAvailability(ctx context.Context, tenantID uuid.UUID, days int16, arrival string, departure *string) ([]models.FleetVehicle, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_fleet_availability(
			p_tenant_id      := $1,
			p_days           := $2::SMALLINT,
			p_arrival_time   := $3::TIME,
			p_departure_time := $4::TIME
		)`, tenantID, days, arrival, departure)
	if err != nil {
		return nil, mapOptimizationError(err, trackingErrors.PlanningFailed)
	}
	defer rows.Close()
	fleet := []models.FleetVehicle{}
	for rows.Next() {
		var v models.FleetVehicle
		if err := rows.Scan(&v.ID, &v.PlateNumber, &v.CompanyID, &v.CompanyName, &v.Seats,
			&v.FreeOutbound, &v.FreeReturn, &v.BusyWith); err != nil {
			return nil, err
		}
		fleet = append(fleet, v)
	}
	if err := rows.Err(); err != nil {
		return nil, mapOptimizationError(err, trackingErrors.PlanningFailed)
	}
	return fleet, nil
}

// CreateOptimizationRun queues a proposal, or answers the same problem's run of the last 24 h.
func (r *TrackingRepository) CreateOptimizationRun(ctx context.Context, tenantID, organizationID uuid.UUID, dto *models.CreateOptimizationRunDto, userID *uuid.UUID) (*models.OptimizationRunCreated, error) {
	params, err := json.Marshal(dto)
	if err != nil {
		return nil, err
	}
	var out models.OptimizationRunCreated
	err = r.dbService.QueryRow(ctx,
		`SELECT run_id, cached FROM tracking.sp_create_optimization_run(p_tenant_id := $1, p_organization_id := $2, p_params := $3::JSONB, p_user_id := $4)`,
		tenantID, organizationID, params, userID).Scan(&out.RunID, &out.Cached)
	if err != nil {
		return nil, mapOptimizationError(err, trackingErrors.OptimizationFailed)
	}
	return &out, nil
}

// GetOptimizationRun answers one run with its result once done.
func (r *TrackingRepository) GetOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID) (*models.OptimizationRun, error) {
	var run models.OptimizationRun
	var result []byte
	err := r.dbService.QueryRow(ctx,
		`SELECT id, organization_id, status, error_code, params, result, applied_at, created_at FROM tracking.sp_get_optimization_run(p_tenant_id := $1, p_run_id := $2)`,
		tenantID, runID).Scan(&run.ID, &run.OrganizationID, &run.Status, &run.ErrorCode, &run.Params, &result, &run.AppliedAt, &run.CreatedAt)
	if err != nil {
		return nil, mapOptimizationError(err, trackingErrors.OptimizationFailed)
	}
	if len(result) > 0 {
		run.OptimizationResult = &models.OptimizationResult{}
		if err := json.Unmarshal(result, run.OptimizationResult); err != nil {
			return nil, err
		}
	}
	return &run, nil
}

// ClaimOptimizationRun marks a queued run running and answers its problem; nil when there is nothing
// left to solve (done, failed or gone).
func (r *TrackingRepository) ClaimOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID) (*models.OptimizationInput, error) {
	var raw []byte
	err := r.dbService.QueryRow(ctx,
		`SELECT input FROM tracking.sp_claim_optimization_run(p_tenant_id := $1, p_run_id := $2)`, tenantID, runID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var input models.OptimizationInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	return &input, nil
}

// FinishOptimizationRun records a run's outcome: done with result, or failed with errorCode.
func (r *TrackingRepository) FinishOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID, status string, errorCode *string, result *models.OptimizationResult) error {
	var raw []byte
	if result != nil {
		var err error
		if raw, err = json.Marshal(result); err != nil {
			return err
		}
	}
	var done int
	return r.dbService.QueryRow(ctx,
		`SELECT 1 FROM tracking.sp_finish_optimization_run(p_tenant_id := $1, p_run_id := $2, p_status := $3, p_error_code := $4, p_result := $5::JSONB)`,
		tenantID, runID, status, errorCode, raw).Scan(&done)
}

// ApplyOptimizationRun creates the proposal's route pairs and assignments in one transaction.
func (r *TrackingRepository) ApplyOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID, dto *models.ApplyOptimizationRunDto, userID *uuid.UUID) (*models.ApplyOptimizationRunResponse, error) {
	routes, err := json.Marshal(dto.Routes)
	if err != nil {
		return nil, err
	}
	var out models.ApplyOptimizationRunResponse
	err = r.dbService.QueryRow(ctx, `
		SELECT routes_created, assignments_created FROM tracking.sp_apply_optimization_run(
			p_tenant_id      := $1,
			p_run_id         := $2,
			p_routes         := $3::JSONB,
			p_effective_from := $4::DATE,
			p_user_id        := $5
		)`, tenantID, runID, routes, dto.EffectiveFrom, userID).Scan(&out.RoutesCreated, &out.AssignmentsCreated)
	if err != nil {
		return nil, mapOptimizationError(err, trackingErrors.OptimizationApplyFailed)
	}
	return &out, nil
}
