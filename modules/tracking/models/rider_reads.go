package models

import (
	"time"

	"github.com/google/uuid"
)

// The guardian's reads of TRACK-061: the rider's day, their history, the absence cut-offs, and the
// account's riders across operators.

// RiderToday is GET /tracking/riders/:id/today (D2): every trip of the rider's day, in order.
type RiderToday struct {
	RiderID      uuid.UUID          `json:"rider_id"`
	RiderName    string             `json:"rider_name"`
	Date         string             `json:"date"`
	Timezone     string             `json:"timezone"`
	Organization RiderOrganization  `json:"organization"`
	Company      *RiderCompany      `json:"company"`
	Absence      *RiderTodayAbsence `json:"absence"`
	Trips        []RiderTodayTrip   `json:"trips"`
}

type RiderOrganization struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// RiderCompany is the carrier the guardian calls.
type RiderCompany struct {
	Name  string  `json:"name"`
	Phone *string `json:"phone"`
}

// RiderTodayAbsence is the absence covering today; Direction nil is both.
type RiderTodayAbsence struct {
	ID        uuid.UUID `json:"id"`
	Direction *string   `json:"direction"`
}

// RiderTodayTrip is one trip carrying the rider. StopsLeft is how many stops the bus makes before the
// rider's next one, nil once the rider has none left.
type RiderTodayTrip struct {
	TripID       uuid.UUID       `json:"trip_id"`
	RouteName    string          `json:"route_name"`
	Direction    string          `json:"direction"`
	Status       string          `json:"status"`
	PlannedStart time.Time       `json:"planned_start"`
	StartedAt    *time.Time      `json:"started_at"`
	EndedAt      *time.Time      `json:"ended_at"`
	LicensePlate *string         `json:"license_plate"`
	DriverName   *string         `json:"driver_name"`
	Pickup       *RiderTodayStop `json:"pickup"`
	Dropoff      *RiderTodayStop `json:"dropoff"`
	StopsLeft    *int32          `json:"stops_left"`
}

// RiderTodayStop is the rider's pickup or dropoff: planned and actual, and the ETA while it applies.
type RiderTodayStop struct {
	TripStopID uuid.UUID  `json:"trip_stop_id"`
	StopName   string     `json:"stop_name"`
	Lat        *float64   `json:"lat"`
	Lng        *float64   `json:"lng"`
	PlannedAt  *time.Time `json:"planned_at"`
	ArrivedAt  *time.Time `json:"arrived_at"`
	TaskStatus string     `json:"task_status"`
	DoneAt     *time.Time `json:"done_at"`
	EtaAt      *time.Time `json:"eta_at"`
}

// RiderDaysQuery binds the from/to of GET /tracking/riders/:id/history and /absence-cutoffs; either
// left out takes the endpoint's default.
type RiderDaysQuery struct {
	From *string `form:"from" binding:"omitempty,datetime=2006-01-02"`
	To   *string `form:"to" binding:"omitempty,datetime=2006-01-02"`
}

// RiderHistory is GET /tracking/riders/:id/history (D3): one row per day, newest first.
type RiderHistory struct {
	RiderID uuid.UUID         `json:"rider_id"`
	From    string            `json:"from"`
	To      string            `json:"to"`
	Days    []RiderHistoryDay `json:"days"`
}

// RiderHistoryDay is a day's summary — rode, partial, no_show, absent or cancelled — and its legs.
type RiderHistoryDay struct {
	Date    string               `json:"date"`
	Status  string               `json:"status"`
	Legs    []RiderHistoryLeg    `json:"legs"`
	Absence *RiderHistoryAbsence `json:"absence"`
}

type RiderHistoryLeg struct {
	TripID      uuid.UUID  `json:"trip_id"`
	Direction   string     `json:"direction"`
	RouteName   string     `json:"route_name"`
	Status      string     `json:"status"`
	PickupStop  *string    `json:"pickup_stop"`
	PickupAt    *time.Time `json:"pickup_at"`
	DropoffStop *string    `json:"dropoff_stop"`
	DropoffAt   *time.Time `json:"dropoff_at"`
}

type RiderHistoryAbsence struct {
	ID        uuid.UUID `json:"id"`
	Direction *string   `json:"direction"`
	Reason    *string   `json:"reason"`
}

// RiderAbsenceCutoffs is GET /tracking/riders/:id/absence-cutoffs: until when each day can still be
// reported, by the rule POST …/absences refuses with cutoff.
type RiderAbsenceCutoffs struct {
	CutoffMin int32            `json:"cutoff_min"`
	Days      []RiderCutoffDay `json:"days"`
}

// RiderCutoffDay: HasTrip false (and CutoffAt nil) when no pickup is materialised that day.
type RiderCutoffDay struct {
	Date     string     `json:"date"`
	HasTrip  bool       `json:"has_trip"`
	CutoffAt *time.Time `json:"cutoff_at"`
	Closed   bool       `json:"closed"`
}

// PortalMe is GET /mobile/portal/me (D1): the account and its riders in every operator it is a
// guardian in, newest membership first.
type PortalMe struct {
	User    PortalUser     `json:"user"`
	Tenants []PortalTenant `json:"tenants"`
}

type PortalUser struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
}

// PortalTenant is one operator: its riders as GET /tracking/riders/today answers them, or Error with
// Riders nil when that operator's read failed.
type PortalTenant struct {
	Slug   string                `json:"slug"`
	Name   string                `json:"name"`
	Riders []RiderStatusResponse `json:"riders"`
	Error  bool                  `json:"error"`
}

// PortalMembership is one tenant /me reads: DatabaseURL empty is the shared database.
type PortalMembership struct {
	TenantID    uuid.UUID
	Slug        string
	Name        string
	DatabaseURL string
}
