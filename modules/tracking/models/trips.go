package models

import (
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// The trip slice of TRACK-020: the list, the detail, the status and what the writes take.

// Trip is the one row shape every trip read and write answers. TasksTotal leaves out cancelled
// tasks; TasksDone counts what is no longer pending (done or no_show).
type Trip struct {
	ID              uuid.UUID           `json:"id"`
	RouteID         uuid.UUID           `json:"route_id"`
	RouteName       string              `json:"route_name"`
	Direction       string              `json:"direction"`
	RouteScheduleID *uuid.UUID          `json:"route_schedule_id"`
	ServiceDate     coreModels.DateOnly `json:"service_date"`
	Timezone        string              `json:"timezone"`
	PlannedStart    time.Time           `json:"planned_start"`
	VehicleID       *uuid.UUID          `json:"vehicle_id"`
	LicensePlate    *string             `json:"license_plate"`
	DriverID        *uuid.UUID          `json:"driver_id"`
	DriverName      *string             `json:"driver_name"`
	Status          string              `json:"status"`
	StartedAt       *time.Time          `json:"started_at"`
	EndedAt         *time.Time          `json:"ended_at"`
	IsOverridden    bool                `json:"is_overridden"`
	StopsCount      int32               `json:"stops_count"`
	TasksTotal      int32               `json:"tasks_total"`
	TasksDone       int32               `json:"tasks_done"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// TripListRow is a row of GET /tracking/trips: the trip plus how long its next pending stop has
// been due (TRACK-021) — 0 when not yet due, nil once the trip has ended.
type TripListRow struct {
	Trip
	DelaySeconds *int32 `json:"delay_seconds"`
}

// TripDetail is GET /tracking/trips/:id and what every trip write answers: the row plus its stops
// in order, each with its tasks. Polyline (precision 6), DistanceKm and TraceSource are the driven
// path stored at trip close (TRACK-010); only the GET reads them, a write answers them null.
type TripDetail struct {
	Trip
	Polyline    *string    `json:"polyline"`
	DistanceKm  *float64   `json:"distance_km"`
	TraceSource *string    `json:"trace_source"`
	Stops       []TripStop `json:"stops"`
}

type TripStop struct {
	ID          uuid.UUID  `json:"id"`
	Sequence    int32      `json:"sequence"`
	StopPlaceID uuid.UUID  `json:"stop_place_id"`
	StopName    string     `json:"stop_name"`
	PlannedAt   *time.Time `json:"planned_at"`
	ArrivedAt   *time.Time `json:"arrived_at"`
	DepartedAt  *time.Time `json:"departed_at"`
	Status      string     `json:"status"`
	Tasks       []TripTask `json:"tasks"`
	// Lat and Lng come with the live snapshot only (MOBILE-010): the map draws the stops from it.
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`
}

type TripTask struct {
	ID          uuid.UUID  `json:"id"`
	Kind        string     `json:"kind"`
	SubjectType string     `json:"subject_type"`
	SubjectID   uuid.UUID  `json:"subject_id"`
	RiderName   *string    `json:"rider_name"`
	Status      string     `json:"status"`
	DoneAt      *time.Time `json:"done_at"`
}

// TripStatus is GET /tracking/trips/:id/status: the counts of the route status over this trip, the
// next pending stop and how long it has been due (D2); both NULL once the trip has ended.
type TripStatus struct {
	TripID             uuid.UUID     `json:"trip_id"`
	RouteID            uuid.UUID     `json:"route_id"`
	Status             string        `json:"status"`
	PlannedStart       time.Time     `json:"planned_start"`
	StartedAt          *time.Time    `json:"started_at"`
	EndedAt            *time.Time    `json:"ended_at"`
	NextStop           *TripNextStop `json:"next_stop"`
	DelaySeconds       *int32        `json:"delay_seconds"`
	VehicleID          *uuid.UUID    `json:"vehicle_id"`
	CurrentLatitude    *float64      `json:"current_latitude"`
	CurrentLongitude   *float64      `json:"current_longitude"`
	CurrentSpeed       *float64      `json:"current_speed"`
	LocationAgeSeconds *int32        `json:"location_age_seconds"`
	TotalRiders        int32         `json:"total_riders"`
	BoardedCount       int32         `json:"boarded_count"`
	ArrivedCount       int32         `json:"arrived_count"`
	NoShowCount        int32         `json:"no_show_count"`
	PendingCount       int32         `json:"pending_count"`
}

type TripNextStop struct {
	ID        uuid.UUID  `json:"id"`
	Sequence  int32      `json:"sequence"`
	StopName  string     `json:"stop_name"`
	PlannedAt *time.Time `json:"planned_at"`
}

// ListTripsQuery binds GET /tracking/trips. Date empty is each route's own today.
type ListTripsQuery struct {
	Date           *string `form:"date" binding:"omitempty,datetime=2006-01-02"`
	RouteID        *string `form:"route_id" binding:"omitempty,uuid"`
	Status         *string `form:"status" binding:"omitempty,oneof=planned in_progress completed cancelled"`
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
	Page           int     `form:"page,default=1" binding:"min=1"`
	PageSize       int     `form:"page_size,default=20" binding:"min=1,max=100"`
	// IncludeEmpty false leaves out the trips nobody rides that have not started (TRACK-053 D1); unset
	// keeps them, so callers other than the web board see every trip.
	IncludeEmpty *bool `form:"include_empty"`
	// ScopeUserID narrows the board to an organization user's trips (MOBILE-020); set by the
	// controller from the access level, never bound from the query.
	ScopeUserID *uuid.UUID `form:"-"`
}

type ListTripsResponse struct {
	Trips      []*TripListRow `json:"trips"`
	TotalCount int64          `json:"total_count"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}

// UpdateTripDto is PATCH /tracking/trips/:id, the operator's override: a field left out keeps its
// value, and any change marks the trip is_overridden.
type UpdateTripDto struct {
	VehicleID    *uuid.UUID `json:"vehicle_id,omitempty" binding:"omitempty"`
	DriverID     *uuid.UUID `json:"driver_id,omitempty" binding:"omitempty"`
	PlannedStart *time.Time `json:"planned_start,omitempty" binding:"omitempty"`
}

// TripTaskTransitionDto is POST /tracking/trip-stop-tasks/:id/done | no-show. The same
// client_op_id sent again is answered without a second transition.
type TripTaskTransitionDto struct {
	ClientOpID *uuid.UUID     `json:"client_op_id" binding:"required"`
	Proof      map[string]any `json:"proof,omitempty"`
}
