package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// The guardian's notices of TRACK-012: the in-app feed, and which pushes each rider's notices mute.

// Notification is one feed row. Payload carries what the phone renders the notice with: trip_id,
// route_name, rider_name, and stop_name or title when the type has one.
type Notification struct {
	ID        uuid.UUID       `json:"id"`
	RiderID   uuid.UUID       `json:"rider_id"`
	RiderName string          `json:"rider_name"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	ReadAt    *time.Time      `json:"read_at"`
}

type ListNotificationsQuery struct {
	Unread   bool `form:"unread"`
	Page     int  `form:"page,default=1" binding:"min=1"`
	PageSize int  `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ListNotificationsResponse is one page of the feed; UnreadCount is the whole badge whatever the
// filter.
type ListNotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	TotalCount    int64          `json:"total_count"`
	Page          int            `json:"page"`
	PageSize      int            `json:"page_size"`
	UnreadCount   int64          `json:"unread_count"`
}

// MarkNotificationsReadDto names the rows to mark read, or all of them.
type MarkNotificationsReadDto struct {
	IDs []uuid.UUID `json:"ids" binding:"required_without=All,max=100"`
	All bool        `json:"all"`
}

// NotificationSetting is one rider's muted notice types; empty is every notice on.
type NotificationSetting struct {
	RiderID   uuid.UUID `json:"rider_id" binding:"required"`
	RiderName string    `json:"rider_name,omitempty"`
	Muted     []string  `json:"muted" binding:"max=9,dive,oneof=trip_started approaching boarded dropped_off no_show cancelled changed delay trip_progress"`
}

// NewNotice is a feed row sp_notify_event just wrote and the account did not mute: one push to send.
type NewNotice struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	RiderID uuid.UUID
	Type    string
	Payload json.RawMessage
}

// TripProgressRow is one guardian's trip-progress push (MOBILE-024): where their rider is in the trip,
// counted up to the rider's own stop — the pickup while waiting, the drop-off on board.
type TripProgressRow struct {
	RiderID          uuid.UUID
	RiderName        string
	UserID           uuid.UUID
	Phase            string // to_pickup | on_board | done
	StopsDone        int
	StopsTotal       int
	CurrentStop      *string
	NextStop         *string
	TargetStop       *string
	TargetTripStopID *uuid.UUID
	TripStatus       string
	VehicleType      *string
	OrganizationKind *string
}
