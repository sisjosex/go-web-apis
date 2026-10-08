package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The live slice of TRACK-025: the snapshots a live screen reads when it opens or resyncs.

// LiveFleetQuery binds GET /tracking/live/fleet.
type LiveFleetQuery struct {
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
}

// LiveVehicle is one reporting vehicle: its last position, and its trip in progress with the route.
type LiveVehicle struct {
	VehicleID    uuid.UUID  `json:"vehicle_id"`
	LicensePlate string     `json:"license_plate"`
	TripID       *uuid.UUID `json:"trip_id"`
	RouteID      *uuid.UUID `json:"route_id"`
	RouteName    *string    `json:"route_name"`
	Lat          float64    `json:"lat"`
	Lng          float64    `json:"lng"`
	Speed        *float32   `json:"speed"`
	Heading      *float32   `json:"heading"`
	RecordedAt   time.Time  `json:"recorded_at"`
	// OrganizationKind is the route client's kind (school | company | other), which picks the marker:
	// nil without a trip or when the route names no client (TRACK-051).
	OrganizationKind *string `json:"organization_kind"`
}

type LiveFleetResponse struct {
	Vehicles []LiveVehicle `json:"vehicles"`
}

// StopEta is when the vehicle is expected at one pending stop, and where the estimate came from
// (valhalla, historical or fallback, as geo names them).
type StopEta struct {
	TripStopID     uuid.UUID `json:"trip_stop_id"`
	EtaAt          time.Time `json:"eta_at"`
	EstimateSource string    `json:"estimate_source"`
}

// TripLive is GET /tracking/trips/:id/live: the detail, the vehicle's last position (the same shape
// a position frame carries) and an ETA per pending stop.
type TripLive struct {
	TripDetail
	Position json.RawMessage `json:"position"`
	Eta      []StopEta       `json:"eta"`
	LiveStory
}

// LiveStory is what a live map draws beyond the stops (MOBILE-023): the planned line, the vehicle's
// type and the client's kind for the marker, and where the trip ends. Each is null when unknown.
type LiveStory struct {
	PlannedPath      *PlannedPath     `json:"planned_path"`
	VehicleType      *string          `json:"vehicle_type"`
	OrganizationKind *string          `json:"organization_kind"`
	Destination      *LiveDestination `json:"destination"`
}

// PlannedPath is the line the trip's route version planned, precision-6 polyline. ETag is its md5:
// a caller that already holds it gets the object without Polyline6 (MOBILE-023 D2).
type PlannedPath struct {
	Polyline6 string `json:"polyline6,omitempty"`
	DistanceM *int   `json:"distance_m"`
	ETag      string `json:"etag"`
}

// LiveDestination is the trip's last located stop.
type LiveDestination struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// DropKnownPath leaves the line out when etag is the one the caller holds.
func (s *LiveStory) DropKnownPath(etag string) {
	if s.PlannedPath != nil && etag != "" && etag == s.PlannedPath.ETag {
		s.PlannedPath.Polyline6 = ""
	}
}

// RiderLive is GET /tracking/riders/:id/live (MOBILE-010): the rider's trip in progress, the rider's own
// stops on it, the vehicle's last position (a position frame's shape) and the ETA to the rider's
// pending stops. No trip in progress: TripID nil, no stops, no position.
type RiderLive struct {
	RiderID      uuid.UUID       `json:"rider_id"`
	TripID       *uuid.UUID      `json:"trip_id"`
	RouteName    *string         `json:"route_name"`
	LicensePlate *string         `json:"license_plate"`
	Stops        []RiderLiveStop `json:"stops"`
	Position     json.RawMessage `json:"position" swaggertype:"object"`
	Eta          []StopEta       `json:"eta"`
	LiveStory
}

// RiderLiveStop is one of the rider's stops on the trip: where it is and whether it is pickup or dropoff.
type RiderLiveStop struct {
	TripStopID uuid.UUID  `json:"trip_stop_id"`
	StopName   string     `json:"stop_name"`
	Lat        float64    `json:"lat"`
	Lng        float64    `json:"lng"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	PlannedAt  *time.Time `json:"planned_at"`
}

// PendingStop is a stop the ETA is computed to, in sequence.
type PendingStop struct {
	TripStopID uuid.UUID `json:"trip_stop_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
}

// LivePosition is the part of a position the ETA reads.
type LivePosition struct {
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	RecordedAt time.Time `json:"recorded_at"`
}
