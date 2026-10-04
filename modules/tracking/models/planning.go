package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The planning screen of TRACK-014: what a destination needs, the fleet free at its hours, and a
// route proposal reviewed before it is applied.

// PlanningQuery is `GET /tracking/organizations/:id/planning`: the weekdays bitmask and the hours the
// proposal would serve; an hour left out is read off the organization's running routes.
type PlanningQuery struct {
	Days          int16   `form:"days,default=31" binding:"min=1,max=127"`
	ArrivalTime   *string `form:"arrival_time" binding:"omitempty,datetime=15:04"`
	DepartureTime *string `form:"departure_time" binding:"omitempty,datetime=15:04"`
}

// PlanningRider is one rider on the planning map: where they are picked up (nil: no pin), the legs
// they contracted and the route they already ride each way.
type PlanningRider struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	Lat               *float64   `json:"lat"`
	Lng               *float64   `json:"lng"`
	ServiceLegs       string     `json:"service_legs"`
	OutboundRouteID   *uuid.UUID `json:"outbound_route_id"`
	OutboundRouteName *string    `json:"outbound_route_name"`
	ReturnRouteID     *uuid.UUID `json:"return_route_id"`
	ReturnRouteName   *string    `json:"return_route_name"`
}

// PlanningDemand is the Demanda card: riders per leg, the ones without a pin and the ones still
// without a route each way.
type PlanningDemand struct {
	Outbound           int `json:"outbound"`
	Return             int `json:"return"`
	MissingLocation    int `json:"missing_location"`
	UnassignedOutbound int `json:"unassigned_outbound"`
	UnassignedReturn   int `json:"unassigned_return"`
}

// FleetVehicle is one vehicle of the Flota card: seats, free each way at the planned hours, and the
// runs that keep it busy ({route_id, route_name, leg, start_time, end_time}).
type FleetVehicle struct {
	ID           uuid.UUID       `json:"id"`
	PlateNumber  string          `json:"plate_number"`
	CompanyID    uuid.UUID       `json:"company_id"`
	CompanyName  string          `json:"company_name"`
	Seats        int             `json:"seats"`
	FreeOutbound bool            `json:"free_outbound"`
	FreeReturn   bool            `json:"free_return"`
	BusyWith     json.RawMessage `json:"busy_with" swaggertype:"array,object"`
}

// PlanningOverview is the whole screen in one answer.
type PlanningOverview struct {
	ArrivalTime   string          `json:"arrival_time"`
	DepartureTime string          `json:"departure_time"`
	Demand        PlanningDemand  `json:"demand"`
	Riders        []PlanningRider `json:"riders"`
	Fleet         []FleetVehicle  `json:"fleet"`
}

// CreateOptimizationRunDto is `POST /tracking/organizations/:id/optimization-runs`.
type CreateOptimizationRunDto struct {
	Days          *int16      `json:"days" binding:"omitempty,min=1,max=127"`
	ArrivalTime   string      `json:"arrival_time" binding:"required,datetime=15:04"`
	DepartureTime *string     `json:"departure_time" binding:"omitempty,datetime=15:04"`
	VehicleIDs    []uuid.UUID `json:"vehicle_ids" binding:"required,min=1,max=200"`
	MaxRideMin    *int        `json:"max_ride_min" binding:"omitempty,min=10,max=240"`
	// MaxRiders caps the batch below the 100 a proposal takes at most.
	MaxRiders *int `json:"max_riders" binding:"omitempty,min=1,max=100"`
}

// OptimizationRunCreated is the 202: the run to poll, and whether it is an earlier one of the same
// problem.
type OptimizationRunCreated struct {
	RunID  uuid.UUID `json:"run_id"`
	Cached bool      `json:"cached"`
}

// OptimizationInput is the problem a run stores: what VROOM is asked.
type OptimizationInput struct {
	Destination struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"destination"`
	ArrivalTime string `json:"arrival_time"`
	MaxRideMin  int    `json:"max_ride_min"`
	// Deferred counts the riders left for the next proposal: beyond the batch this one takes.
	Deferred int `json:"deferred"`
	Riders   []struct {
		ID  uuid.UUID `json:"id"`
		Lat float64   `json:"lat"`
		Lng float64   `json:"lng"`
	} `json:"riders"`
	Vehicles []struct {
		ID    uuid.UUID `json:"id"`
		Seats int       `json:"seats"`
	} `json:"vehicles"`
}

// ProposedStop is one pickup of a proposed route and the time the vehicle is expected there.
type ProposedStop struct {
	RiderID uuid.UUID `json:"rider_id"`
	Lat     float64   `json:"lat"`
	Lng     float64   `json:"lng"`
	Eta     string    `json:"eta"`
}

// ProposedRoute is one vehicle of the proposal: its riders in pickup order, how far it drives and the
// longest ride on board (the first rider's).
type ProposedRoute struct {
	VehicleID uuid.UUID      `json:"vehicle_id"`
	RiderIDs  []uuid.UUID    `json:"rider_ids"`
	Stops     []ProposedStop `json:"stops"`
	Km        float64        `json:"km"`
	RideMin   int            `json:"ride_min"`
}

// UnassignedRider is a rider no vehicle took: `capacity` (not enough seats) or `max-ride` (too far
// for the longest ride allowed).
type UnassignedRider struct {
	RiderID uuid.UUID `json:"rider_id"`
	Reason  string    `json:"reason"`
}

// OptimizationMetrics sums the proposal up.
type OptimizationMetrics struct {
	VehiclesUsed int     `json:"vehicles_used"`
	TotalKm      float64 `json:"total_km"`
	MaxRideMin   int     `json:"max_ride_min"`
	// Deferred riders wait for the next proposal, once this one is applied.
	Deferred int `json:"deferred"`
}

// OptimizationResult is a done run's answer.
type OptimizationResult struct {
	Metrics    OptimizationMetrics `json:"metrics"`
	Routes     []ProposedRoute     `json:"routes"`
	Unassigned []UnassignedRider   `json:"unassigned"`
}

// OptimizationRun is `GET /tracking/optimization-runs/:id`; a done run carries its result's fields.
type OptimizationRun struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Status         string          `json:"status"`
	ErrorCode      *string         `json:"error_code,omitempty"`
	Params         json.RawMessage `json:"params" swaggertype:"object"`
	AppliedAt      *time.Time      `json:"applied_at"`
	CreatedAt      time.Time       `json:"created_at"`
	*OptimizationResult
}

// ApplyOptimizationRoute is one route as the operator left it: a vehicle and its riders in order.
type ApplyOptimizationRoute struct {
	VehicleID uuid.UUID   `json:"vehicle_id" binding:"required"`
	RiderIDs  []uuid.UUID `json:"rider_ids" binding:"required,min=1,max=100"`
}

// ApplyOptimizationRunDto is `POST /tracking/optimization-runs/:id/apply`.
type ApplyOptimizationRunDto struct {
	EffectiveFrom string                   `json:"effective_from" binding:"required,datetime=2006-01-02"`
	Routes        []ApplyOptimizationRoute `json:"routes" binding:"required,min=1,max=200,dive"`
}

// ApplyOptimizationRunResponse is what applying created.
type ApplyOptimizationRunResponse struct {
	RoutesCreated      int `json:"routes_created"`
	AssignmentsCreated int `json:"assignments_created"`
}
