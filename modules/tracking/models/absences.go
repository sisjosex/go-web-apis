package models

import (
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// The absence and suggestion slice of TRACK-022.

// RiderAbsence is a date range a rider does not ride, both directions when Direction is nil.
// ReportedVia is `portal` for a guardian's report and `web` for anyone else's (D3).
type RiderAbsence struct {
	ID             uuid.UUID           `json:"id"`
	RiderID        uuid.UUID           `json:"rider_id"`
	DateFrom       coreModels.DateOnly `json:"date_from"`
	DateTo         coreModels.DateOnly `json:"date_to"`
	Direction      *string             `json:"direction"`
	Reason         *string             `json:"reason"`
	ReportedBy     *uuid.UUID          `json:"reported_by"`
	ReportedByName *string             `json:"reported_by_name"`
	ReportedVia    string              `json:"reported_via"`
	CreatedAt      time.Time           `json:"created_at"`
}

// ListRiderAbsencesQuery binds GET /tracking/riders/:id/absences: the absences overlapping
// [from, to], either bound open when empty.
type ListRiderAbsencesQuery struct {
	From *string `form:"from" binding:"omitempty,datetime=2006-01-02"`
	To   *string `form:"to" binding:"omitempty,datetime=2006-01-02"`
}

// CreateRiderAbsenceDto is POST /tracking/riders/:id/absences. Direction empty is both.
type CreateRiderAbsenceDto struct {
	DateFrom  string  `json:"date_from" binding:"required,datetime=2006-01-02"`
	DateTo    string  `json:"date_to" binding:"required,datetime=2006-01-02"`
	Direction *string `json:"direction" binding:"omitempty,oneof=outbound inbound"`
	Reason    *string `json:"reason" binding:"omitempty,max=500" conform:"trim"`
}

// CreateRiderAbsenceResponse answers the absence and how many pending tasks it cancelled (D2).
type CreateRiderAbsenceResponse struct {
	Absence        *RiderAbsence `json:"absence"`
	CancelledTasks int32         `json:"cancelled_tasks"`
}

// SuggestRiderStopsQuery binds GET /tracking/riders/:id/suggestions. Days is the weekday bitmask
// the rider would ride (bit 0 Monday); MaxWalkM the straight-line walk from home (D3).
type SuggestRiderStopsQuery struct {
	Direction string `form:"direction" binding:"required,oneof=outbound inbound"`
	Days      int16  `form:"days,default=127" binding:"min=1,max=127"`
	MaxWalkM  int32  `form:"max_walk_m,default=800" binding:"min=1,max=3000"`
	Limit     int32  `form:"limit,default=10" binding:"min=1,max=50"`
}

// RiderStopSuggestion is a stop near the rider's home on a route of the direction asked. Capacity
// and FreeSeats are nil when the route has no vehicle.
type RiderStopSuggestion struct {
	RouteID       uuid.UUID `json:"route_id"`
	RouteName     string    `json:"route_name"`
	Direction     string    `json:"direction"`
	StopPlaceID   uuid.UUID `json:"stop_place_id"`
	StopName      string    `json:"stop_name"`
	Latitude      *float64  `json:"latitude"`
	Longitude     *float64  `json:"longitude"`
	WalkDistanceM int32     `json:"walk_distance_m"`
	Capacity      *int32    `json:"capacity"`
	Assigned      int32     `json:"assigned"`
	FreeSeats     *int32    `json:"free_seats"`
}

type SuggestRiderStopsResponse struct {
	Suggestions []*RiderStopSuggestion `json:"suggestions"`
}
