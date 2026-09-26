package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The driver's phone of TRACK-011: its day in one read, and the queue of what it did offline.

// MaxSyncOps caps one POST /mobile/driver/sync body.
const MaxSyncOps = 100

// The ops a phone may queue, and the args key each one's target sits under.
const (
	DriverOpStopArrive   = "stop.arrive"
	DriverOpStopSkip     = "stop.skip"
	DriverOpTaskDone     = "task.done"
	DriverOpTaskNoShow   = "task.no_show"
	DriverOpTripComplete = "trip.complete"
)

// What a synced op answers: applied now, replayed (applied by an earlier sync, nothing ran) or rejected
// with a code — a rejection replayed is rejected again with the same code.
const (
	DriverOpApplied  = "applied"
	DriverOpReplayed = "replayed"
	DriverOpRejected = "rejected"
)

// DriverToday is GET /mobile/driver/today: the driver's trips of each route's local today, each with
// its stops → tasks → rider, as sp_driver_today builds it. Trips passes through unparsed.
type DriverToday struct {
	Date        string          `json:"date"`
	Driver      DriverRef       `json:"driver"`
	Trips       json.RawMessage `json:"trips" swaggertype:"array,object"`
	GeneratedAt time.Time       `json:"generated_at"`
}

// DriverRef is who the account drives as.
type DriverRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// DriverSyncArgs is an op's target: trip_stop_id for a stop op, trip_stop_task_id (and an optional
// proof) for a task op, trip_id for trip.complete.
type DriverSyncArgs struct {
	TripStopID     *uuid.UUID     `json:"trip_stop_id,omitempty"`
	TripStopTaskID *uuid.UUID     `json:"trip_stop_task_id,omitempty"`
	TripID         *uuid.UUID     `json:"trip_id,omitempty"`
	Proof          map[string]any `json:"proof,omitempty"`
}

// DriverSyncOp is one queued op, stamped by the phone when the driver did it.
type DriverSyncOp struct {
	ClientOpID       *uuid.UUID     `json:"client_op_id" binding:"required"`
	Op               string         `json:"op" binding:"required,oneof=stop.arrive stop.skip task.done task.no_show trip.complete"`
	Args             DriverSyncArgs `json:"args"`
	ClientRecordedAt *time.Time     `json:"client_recorded_at" binding:"required"`
}

// DriverSyncDto is POST /mobile/driver/sync.
type DriverSyncDto struct {
	Ops []DriverSyncOp `json:"ops" binding:"required,min=1,max=100,dive"`
}

// DriverOpResult is one op's answer; Code is the module code of a rejection.
type DriverOpResult struct {
	ClientOpID uuid.UUID  `json:"client_op_id"`
	Status     string     `json:"status"`
	Code       *string    `json:"code"`
	TripID     *uuid.UUID `json:"trip_id"`
}

// DriverSyncResponse is the sync's 200 body, one result per op in the order they were applied.
type DriverSyncResponse struct {
	Results []DriverOpResult `json:"results"`
}

// The rider card and the incident report of TRACK-027.

// RiderQRToken is GET|POST /tracking/riders/:id/qr-token: the code printed on the rider's card.
type RiderQRToken struct {
	QRToken string `json:"qr_token"`
}

// DriverRiderTask is a rider's pending pickup on the driver's trip in progress.
type DriverRiderTask struct {
	ID         uuid.UUID `json:"id"`
	TripID     uuid.UUID `json:"trip_id"`
	TripStopID uuid.UUID `json:"trip_stop_id"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
}

// DriverRider is GET /mobile/driver/riders/:code, and one match of the search: the rider and their
// pending pickup, null when they have none on the driver's trip in progress.
type DriverRider struct {
	Rider DriverRef        `json:"rider"`
	Task  *DriverRiderTask `json:"task"`
}

// DriverFindRidersQuery is GET /mobile/driver/riders: part of a name, or a card code typed by hand.
type DriverFindRidersQuery struct {
	Q string `form:"q" binding:"required,min=2,max=100"`
}

// DriverFindRidersResponse is the search's 200 body, at most 20 riders by name.
type DriverFindRidersResponse struct {
	Riders []DriverRider `json:"riders"`
}

// DriverIncidentDto is POST /mobile/driver/incidents: what happened on one of the driver's trips, and
// where the phone was.
type DriverIncidentDto struct {
	TripID    *uuid.UUID `json:"trip_id" binding:"required"`
	AlertType string     `json:"alert_type" binding:"required,oneof=delay breakdown emergency" enums:"delay,breakdown,emergency"`
	Message   string     `json:"message" binding:"max=1000"`
	Lat       *float64   `json:"lat" binding:"required,min=-90,max=90"`
	Lng       *float64   `json:"lng" binding:"required,min=-180,max=180"`
}

// DriverIncident is the alert an incident raised: a route alert naming its trip and position.
type DriverIncident struct {
	ID        uuid.UUID  `json:"id"`
	RouteID   uuid.UUID  `json:"route_id"`
	VehicleID *uuid.UUID `json:"vehicle_id"`
	TripID    uuid.UUID  `json:"trip_id"`
	AlertType string     `json:"alert_type"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Severity  string     `json:"severity"`
	Status    string     `json:"status"`
	Lat       float64    `json:"lat"`
	Lng       float64    `json:"lng"`
	CreatedBy uuid.UUID  `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
}

// DriverIncidentResponse is the incident's 201 body.
type DriverIncidentResponse struct {
	Alert DriverIncident `json:"alert"`
}
