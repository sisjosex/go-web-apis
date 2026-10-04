package services

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// PlanningOverview is the planning screen in one answer (TRACK-014 D2): the destination's riders and
// the fleet at the planned hours — an hour left out is the organization's running one — with the
// Demanda card summed over the riders already read, so the counts never need a query of their own.
func (s *TrackingService) PlanningOverview(ctx context.Context, tenantID, organizationID uuid.UUID, query *models.PlanningQuery) (*models.PlanningOverview, error) {
	arrival, departure, err := s.planningHours(ctx, tenantID, organizationID, query)
	if err != nil {
		return nil, err
	}
	riders, err := s.trackingRepo.PlanningRiders(ctx, tenantID, organizationID, query.Days)
	if err != nil {
		return nil, err
	}
	fleet, err := s.trackingRepo.FleetAvailability(ctx, tenantID, query.Days, arrival, &departure)
	if err != nil {
		return nil, err
	}
	return &models.PlanningOverview{
		ArrivalTime: arrival, DepartureTime: departure, Demand: planningDemand(riders), Riders: riders, Fleet: fleet,
	}, nil
}

// planningHours is the query's hours, the organization's own read only when one is missing.
func (s *TrackingService) planningHours(ctx context.Context, tenantID, organizationID uuid.UUID, query *models.PlanningQuery) (string, string, error) {
	if query.ArrivalTime != nil && query.DepartureTime != nil {
		return *query.ArrivalTime, *query.DepartureTime, nil
	}
	arrival, departure, err := s.trackingRepo.PlanningDefaults(ctx, tenantID, organizationID)
	if err != nil {
		return "", "", err
	}
	if query.ArrivalTime != nil {
		arrival = *query.ArrivalTime
	}
	if query.DepartureTime != nil {
		departure = *query.DepartureTime
	}
	return arrival, departure, nil
}

// planningDemand counts each leg's riders, the ones without a pin and the ones still without a route
// that way.
func planningDemand(riders []models.PlanningRider) models.PlanningDemand {
	var d models.PlanningDemand
	for _, r := range riders {
		if r.Lat == nil {
			d.MissingLocation++
		}
		if r.ServiceLegs != "return" {
			d.Outbound++
			if r.OutboundRouteID == nil {
				d.UnassignedOutbound++
			}
		}
		if r.ServiceLegs != "outbound" {
			d.Return++
			if r.ReturnRouteID == nil {
				d.UnassignedReturn++
			}
		}
	}
	return d
}

// CreateOptimizationRun queues a route proposal, or answers the same problem's recent one.
func (s *TrackingService) CreateOptimizationRun(ctx context.Context, tenantID, organizationID uuid.UUID, dto *models.CreateOptimizationRunDto, userID *uuid.UUID) (*models.OptimizationRunCreated, error) {
	return s.trackingRepo.CreateOptimizationRun(ctx, tenantID, organizationID, dto, userID)
}

func (s *TrackingService) GetOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID) (*models.OptimizationRun, error) {
	return s.trackingRepo.GetOptimizationRun(ctx, tenantID, runID)
}

// ApplyOptimizationRun creates the proposal's routes and assignments, all or nothing (TRACK-014 D4).
func (s *TrackingService) ApplyOptimizationRun(ctx context.Context, tenantID, runID uuid.UUID, dto *models.ApplyOptimizationRunDto, userID *uuid.UUID) (*models.ApplyOptimizationRunResponse, error) {
	return s.trackingRepo.ApplyOptimizationRun(ctx, tenantID, runID, dto, userID)
}
