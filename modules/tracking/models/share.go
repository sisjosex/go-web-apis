package models

import (
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// The share links of TRACK-060.

// CreateShareLinkDto is POST /tracking/trips/:id/share-links and /tracking/riders/:id/share-links.
// TTLMinutes nil lasts until the trip's planned end + 60 minutes; anything above 1440 is cut to it.
type CreateShareLinkDto struct {
	TTLMinutes *int32 `json:"ttl_minutes" binding:"omitempty,min=1,max=1440"`
}

// ShareLinkCreated is the stored link and the token made for it, which exists nowhere else.
type ShareLinkCreated struct {
	ID        uuid.UUID
	TripID    uuid.UUID
	Token     string
	ExpiresAt time.Time
}

// CreateShareLinkResponse answers a new link: the page that opens it and until when.
type CreateShareLinkResponse struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ShareLink is one open link of a trip; CreatedVia is `web` (operator) or `portal` (guardian).
type ShareLink struct {
	ID            uuid.UUID `json:"id"`
	CreatedByName *string   `json:"created_by_name"`
	CreatedVia    string    `json:"created_via"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListShareLinksResponse is GET /tracking/trips/:id/share-links.
type ListShareLinksResponse struct {
	ShareLinks []*ShareLink `json:"share_links"`
}

// SharedStop is one stop of a shared trip; EtaAt is the cached estimate while the stop is pending.
type SharedStop struct {
	TripStopID uuid.UUID  `json:"trip_stop_id"`
	Sequence   int32      `json:"sequence"`
	StopName   string     `json:"stop_name"`
	Lat        *float64   `json:"lat"`
	Lng        *float64   `json:"lng"`
	PlannedAt  *time.Time `json:"planned_at"`
	ArrivedAt  *time.Time `json:"arrived_at"`
	Status     string     `json:"status"`
	EtaAt      *time.Time `json:"eta_at"`
}

// SharedPosition is the bus on a shared trip.
type SharedPosition struct {
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Heading    *float64  `json:"heading"`
	Speed      *float64  `json:"speed"`
	RecordedAt time.Time `json:"recorded_at"`
}

// SharedTrip is GET /public/share/:token (D1): Scope `trip` is the operator's link, `rider` a
// guardian's, which alone carries RiderFirstName. No driver or passenger name is ever in it.
type SharedTrip struct {
	Scope          string              `json:"scope"`
	Status         string              `json:"status"`
	RouteName      string              `json:"route_name"`
	LicensePlate   *string             `json:"license_plate"`
	Direction      *string             `json:"direction"`
	ServiceDate    coreModels.DateOnly `json:"service_date"`
	Timezone       string              `json:"timezone"`
	PlannedStart   time.Time           `json:"planned_start"`
	StartedAt      *time.Time          `json:"started_at"`
	EndedAt        *time.Time          `json:"ended_at"`
	RiderFirstName *string             `json:"rider_first_name,omitempty"`
	Stops          []SharedStop        `json:"stops"`
	Position       *SharedPosition     `json:"position"`
	PlannedPath    *PlannedPath        `json:"planned_path"`
	Destination    *LiveDestination    `json:"destination"`
	ExpiresAt      time.Time           `json:"expires_at"`
	TripID         uuid.UUID           `json:"-"`
}
