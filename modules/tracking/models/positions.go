package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The GPS slice of TRACK-010: what a device or a driver's phone posts, and what one stored batch
// answers per vehicle.

// MaxIngestPoints caps one POST /tracking/ingest/positions body.
const MaxIngestPoints = 500

// IngestFutureSkew is how far ahead of the server's clock a point may be stamped.
const IngestFutureSkew = 5 * time.Minute

// PositionPoint is one point of the body. Seq is the client's own counter, accepted and not stored:
// (vehicle, recorded_at) is what makes a replay a no-op.
type PositionPoint struct {
	RecordedAt time.Time `json:"recorded_at" binding:"required"`
	Lat        *float64  `json:"lat" binding:"required,latitude"`
	Lng        *float64  `json:"lng" binding:"required,longitude"`
	Speed      *float64  `json:"speed" binding:"omitempty,min=0,max=150"`
	Heading    *float64  `json:"heading" binding:"omitempty,min=0,max=360"`
	Accuracy   *float64  `json:"accuracy" binding:"omitempty,min=0,max=10000"`
	Seq        *int64    `json:"seq"`
}

// OsmandQuery is one point of the OsmAnd / Traccar Client protocol (TRACK-068 D1): the device token as
// id, lon for the longitude, timestamp in unix seconds or RFC3339, speed in knots, bearing in degrees.
// altitude and batt are read and ignored.
type OsmandQuery struct {
	ID        string   `form:"id" binding:"required" doc:"The device token issued from the vehicle page"`
	Lat       *float64 `form:"lat" binding:"required"`
	Lon       *float64 `form:"lon" binding:"required"`
	Timestamp string   `form:"timestamp" doc:"Unix seconds or RFC3339; now when absent"`
	Speed     *float64 `form:"speed" doc:"Knots"`
	Bearing   *float64 `form:"bearing"`
	Accuracy  *float64 `form:"accuracy" doc:"Metres"`
	Altitude  *float64 `form:"altitude"`
	Batt      *float64 `form:"batt"`
}

// KnotsToMS turns an OsmAnd speed into the m/s a stored point carries.
const KnotsToMS = 0.514444

// IngestRadii are the two distances a stored point is checked against, in metres: within ArrivalM of
// the trip's next stop marks it arrived (TRACK-010), within ApproachM announces it once (TRACK-012 D4).
// PhonePriorityS is how long a phone fix keeps a device's points out of a trip (TRACK-068 D2).
type IngestRadii struct {
	ArrivalM       int
	ApproachM      int
	PhonePriorityS int
}

// StoredPoint is a point as the stream and sp_ingest_positions take it: the vehicle is the server's
// answer, never the client's.
type StoredPoint struct {
	VehicleID  uuid.UUID `json:"vehicle_id"`
	RecordedAt time.Time `json:"recorded_at"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Speed      *float64  `json:"speed,omitempty"`
	Heading    *float64  `json:"heading,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	Source     string    `json:"source"`
}

// The two receivers a point comes from (TRACK-068 D2): the driver's phone, or a tracker on the bus.
const (
	PointSourcePhone  = "phone"
	PointSourceDevice = "device"
)

// Where an accepted batch went: the stream (the worker stores it), or the database directly because
// Valkey was not there (D2).
const (
	IngestPathQueued = "queued"
	IngestPathStored = "stored"
)

// IngestResponse is the 202 body.
type IngestResponse struct {
	Accepted int    `json:"accepted"`
	Path     string `json:"path"`
}

// IngestedVehicle is one vehicle of a stored batch: its trip in progress, its last position (the
// frame a position publish carries), the riders of that trip and their organizations, and the stop
// the batch arrived at.
type IngestedVehicle struct {
	VehicleID       uuid.UUID       `json:"vehicle_id"`
	TripID          *uuid.UUID      `json:"trip_id"`
	Last            json.RawMessage `json:"last"`
	RiderIDs        []uuid.UUID     `json:"rider_ids"`
	OrganizationIDs []uuid.UUID     `json:"organization_ids"`
	ArrivedStopID   *uuid.UUID      `json:"arrived_stop_id"`
}
