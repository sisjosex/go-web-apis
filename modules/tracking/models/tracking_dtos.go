package models

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// === DTOs for Locations ===

// CurrentLocationResponse represents current vehicle location
type CurrentLocationResponse struct {
	VehicleID    uuid.UUID `json:"vehicle_id"`
	LicensePlate string    `json:"license_plate"`
	Latitude     *float64  `json:"latitude"`
	Longitude    *float64  `json:"longitude"`
	Speed        *float64  `json:"speed"`
	Heading      *float64  `json:"heading"`
	RecordedAt   *string   `json:"recorded_at"`
	AgeSeconds   *int32    `json:"age_seconds"` // How old is this location data
}

// === DTOs for Ride Events ===

// === DTOs for Route Status ===

// RouteRealtimeStatusResponse represents comprehensive route status
type RouteRealtimeStatusResponse struct {
	RouteID      uuid.UUID  `json:"route_id"`
	RouteName    string     `json:"route_name"`
	VehicleID    *uuid.UUID `json:"vehicle_id"`
	LicensePlate *string    `json:"license_plate"`
	// DriverName is the route's default driver (TRACK-006); NULL while the route has none.
	DriverName         *string  `json:"driver_name"`
	CurrentLatitude    *float64 `json:"current_latitude"`
	CurrentLongitude   *float64 `json:"current_longitude"`
	CurrentSpeed       *float64 `json:"current_speed"`
	LocationAgeSeconds *int32   `json:"location_age_seconds"`
	TotalRiders        int32    `json:"total_riders"`
	BoardedCount       int32    `json:"boarded_count"`
	ArrivedCount       int32    `json:"arrived_count"`
	NoShowCount        int32    `json:"no_show_count"`
	PendingCount       int32    `json:"pending_count"`
	ActiveAlerts       int32    `json:"active_alerts"`
}

// === DTOs for Rider Status (Guardian View) ===

// RiderStatusResponse represents status for parent/guardian notifications
type RiderStatusResponse struct {
	RiderID              uuid.UUID  `json:"rider_id"`
	RiderName            string     `json:"rider_name"`
	RouteID              *uuid.UUID `json:"route_id"`
	RouteName            *string    `json:"route_name"`
	VehicleID            *uuid.UUID `json:"vehicle_id"`
	LicensePlate         *string    `json:"license_plate"`
	DriverName           *string    `json:"driver_name"`
	VehicleLatitude      *float64   `json:"vehicle_latitude"`
	VehicleLongitude     *float64   `json:"vehicle_longitude"`
	VehicleSpeed         *float64   `json:"vehicle_speed"`
	LocationAgeSeconds   *int32     `json:"location_age_seconds"`
	LastEventType        *string    `json:"last_event_type"`
	LastEventTime        *string    `json:"last_event_time"`
	LastEventNotes       *string    `json:"last_event_notes"`
	LastEventStop        *string    `json:"last_event_stop"`
	ScheduledPickupStop  *string    `json:"scheduled_pickup_stop"`
	ScheduledDropoffStop *string    `json:"scheduled_dropoff_stop"`
	ActiveAlerts         int32      `json:"active_alerts"`
}

// === DTOs for Alerts ===

// CreateAlertDto represents request to create route alert
type CreateAlertDto struct {
	RouteID               uuid.UUID  `json:"route_id" binding:"required,uuidv4"`
	VehicleID             *uuid.UUID `json:"vehicle_id" binding:"omitempty,uuidv4"`
	AlertType             string     `json:"alert_type" binding:"required,oneof=delay breakdown cancellation emergency other" enums:"delay,breakdown,cancellation,emergency,other"`
	Title                 string     `json:"title" binding:"required,min=3,max=255" conform:"trim"`
	Message               string     `json:"message" binding:"required,min=5,max=1000" conform:"trim"`
	Severity              string     `json:"severity" binding:"omitempty,oneof=low medium high critical"`
	EstimatedDelayMinutes *int32     `json:"estimated_delay_minutes" binding:"omitempty,min=1,max=999"`
}

// AlertCreatedResponse represents response after creating alert
type AlertCreatedResponse struct {
	AlertID               uuid.UUID  `json:"alert_id"`
	RouteID               uuid.UUID  `json:"route_id"`
	VehicleID             *uuid.UUID `json:"vehicle_id"`
	AlertType             string     `json:"alert_type"`
	Title                 string     `json:"title"`
	Message               string     `json:"message"`
	Severity              string     `json:"severity"`
	EstimatedDelayMinutes *int32     `json:"estimated_delay_minutes"`
	Status                string     `json:"status"`
	CreatedBy             uuid.UUID  `json:"created_by"`
	CreatedAt             string     `json:"created_at"`
}

// === DTOs for Company Management ===

// CreateCompanyDto represents request to create transport company
type CreateCompanyDto struct {
	Name               string  `json:"name" binding:"required,min=3,max=255" conform:"trim"`
	Email              string  `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Phone              string  `json:"phone" binding:"required,max=20" conform:"trim"`
	Address            string  `json:"address" binding:"required,max=500" conform:"trim"`
	City               string  `json:"city" binding:"max=100" conform:"trim"`
	Country            string  `json:"country" binding:"max=100" conform:"trim"`
	RegistrationNumber string  `json:"registration_number" binding:"max=100" conform:"trim"`
	Status             *string `json:"status" binding:"omitempty,oneof=active inactive suspended" conform:"trim,lowercase"`
}

// UpdateCompanyDto represents request to update transport company
type UpdateCompanyDto struct {
	Name    *string `json:"name" binding:"omitempty,min=3,max=255" conform:"trim"`
	Email   *string `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Phone   *string `json:"phone" binding:"omitempty,max=20" conform:"trim"`
	Address *string `json:"address" binding:"omitempty,max=500" conform:"trim"`
	City    *string `json:"city" binding:"omitempty,max=100" conform:"trim"`
	Country *string `json:"country" binding:"omitempty,max=100" conform:"trim"`
}

// ListCompaniesQuery binds GET /tracking/companies (TRACK-001). search matches the name or the
// registration number; status narrows to one lifecycle state.
type ListCompaniesQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status" binding:"omitempty,oneof=active inactive suspended"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListCompaniesResponse struct {
	Companies  []*TransportCompany `json:"companies"`
	TotalCount int64               `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}

// === DTOs for Vehicle Management ===

// ListVehiclesQuery binds GET /tracking/vehicles (TRACK-001 D2). company_id is optional: unset
// lists the whole tenant fleet. It is validated here so a malformed one is a 400, never a
// silently unfiltered list.
type ListVehiclesQuery struct {
	CompanyID   *string `form:"company_id" binding:"omitempty,uuid"`
	VehicleType string  `form:"vehicle_type" binding:"omitempty,oneof=bus van car"`
	Status      string  `form:"status" binding:"omitempty,oneof=active inactive maintenance"`
	Search      string  `form:"search"`
	Page        int     `form:"page,default=1" binding:"min=1"`
	PageSize    int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListVehiclesResponse struct {
	Vehicles   []*Vehicle `json:"vehicles"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// CreateVehicleDto represents request to create vehicle
type CreateVehicleDto struct {
	CompanyID   uuid.UUID `json:"company_id" binding:"required,uuidv4"`
	PlateNumber string    `json:"plate_number" binding:"required,min=3,max=50" conform:"trim,uppercase"`
	VehicleType string    `json:"vehicle_type" binding:"required,oneof=bus van car"`
	Brand       *string   `json:"brand" binding:"omitempty,max=100" conform:"trim"`
	Model       *string   `json:"model" binding:"omitempty,max=100" conform:"trim"`
	Year        *int32    `json:"year" binding:"omitempty,min=1900,max=2100"`
	Capacity    int32     `json:"capacity" binding:"required,min=1,max=200"`
	GPSDeviceID *string   `json:"gps_device_id" binding:"omitempty,max=100" conform:"trim,uppercase"`
	Status      string    `json:"status" binding:"required,oneof=active inactive maintenance"`
}

// UpdateVehicleDto represents request to update vehicle
type UpdateVehicleDto struct {
	PlateNumber *string `json:"plate_number" binding:"omitempty,min=3,max=50" conform:"trim,uppercase"`
	Brand       *string `json:"brand" binding:"omitempty,max=100" conform:"trim"`
	Model       *string `json:"model" binding:"omitempty,max=100" conform:"trim"`
	Year        *int32  `json:"year" binding:"omitempty,min=1900,max=2100"`
	Capacity    *int32  `json:"capacity" binding:"omitempty,min=1,max=200"`
	GPSDeviceID *string `json:"gps_device_id" binding:"omitempty,max=100" conform:"trim,uppercase"`
	Status      *string `json:"status" binding:"omitempty,oneof=active inactive maintenance"`
}

// === DTOs for Rider Management ===

// ListRidersQuery binds GET /tracking/riders (TRACK-005 D1). organization_id is optional: unset
// lists every rider the tenant has. It is validated here so a malformed one is a 400, never a
// silently unfiltered list.
type ListRidersQuery struct {
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
	RiderType      string  `form:"rider_type" binding:"omitempty,oneof=student employee"`
	IsActive       *bool   `form:"is_active"`
	Search         string  `form:"search"`
	Page           int     `form:"page,default=1" binding:"min=1"`
	PageSize       int     `form:"page_size,default=20" binding:"min=1,max=100"`
	// RouteID names, per rider, the route it already rides in that route's direction; Unassigned keeps
	// the riders with none; Group narrows to one course (TRACK-039).
	RouteID    *string `form:"route_id" binding:"omitempty,uuid"`
	Unassigned *bool   `form:"unassigned"`
	Group      string  `form:"group" binding:"omitempty,max=100"`
}

// ListRiderGroupsQuery binds GET /tracking/riders/groups.
type ListRiderGroupsQuery struct {
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
}

// BulkAssignRidersDto is POST /tracking/routes/:id/assignments/bulk (TRACK-039 D1): days and start
// default to the route's schedule days and its today.
type BulkAssignRidersDto struct {
	RiderIDs   []uuid.UUID          `json:"rider_ids" binding:"required,min=1,max=100"`
	DaysOfWeek *int16               `json:"days_of_week" binding:"omitempty,min=1,max=127"`
	ValidFrom  *coreModels.DateOnly `json:"valid_from" time_format:"2006-01-02"`
}

// BulkAssignResult is one rider's outcome: assigned, or skipped with its reason (no-home, overlap…).
type BulkAssignResult struct {
	RiderID uuid.UUID `json:"rider_id"`
	Status  string    `json:"status"`
	Reason  *string   `json:"reason"`
}

// BulkAssignRidersResponse answers every rider, how many went in, the route's seats and the capacity
// warnings of the route and its return.
type BulkAssignRidersResponse struct {
	Results       []*BulkAssignResult `json:"results"`
	AssignedCount int                 `json:"assigned_count"`
	Seats         *int32              `json:"seats"`
	Warnings      []AssignmentWarning `json:"warnings"`
}

type ListRidersResponse struct {
	Riders     []*Rider `json:"riders"`
	TotalCount int64    `json:"total_count"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

// CreateRiderDto represents request to create rider
type CreateRiderDto struct {
	OrganizationID        uuid.UUID `json:"organization_id" binding:"required,uuidv4"`
	RiderType             string    `json:"rider_type" binding:"required,oneof=student employee"`
	FirstName             string    `json:"first_name" binding:"required,min=2,max=255" conform:"trim"`
	LastName              string    `json:"last_name" binding:"required,min=2,max=255" conform:"trim"`
	IdentificationNumber  *string   `json:"identification_number" binding:"omitempty,max=100" conform:"trim"`
	Phone                 *string   `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	Email                 *string   `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	EmergencyContactName  *string   `json:"emergency_contact_name" binding:"omitempty,max=255" conform:"trim"`
	EmergencyContactPhone *string   `json:"emergency_contact_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianName          *string   `json:"guardian_name" binding:"omitempty,max=255" conform:"trim"`
	GuardianPhone         *string   `json:"guardian_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianEmail         *string   `json:"guardian_email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Address               *string   `json:"address" binding:"omitempty,max=500" conform:"trim"`
	HomeLatitude          *float64  `json:"home_latitude" binding:"required_with=HomeLongitude,omitempty,min=-90,max=90"`
	HomeLongitude         *float64  `json:"home_longitude" binding:"required_with=HomeLatitude,omitempty,min=-180,max=180"`
	Notes                 *string   `json:"notes" binding:"omitempty,max=2000" conform:"trim"`
	GroupLabel            *string   `json:"group_label" binding:"omitempty,max=100" conform:"trim"`
	// Guardians are linked in the statement that creates the rider (TRACK-037 D2).
	Guardians []NewRiderGuardianDto `json:"guardians" binding:"omitempty,max=5,dive"`
	// LinkedGuardians is Guardians once their accounts are granted, as fn_rider_guardians_link reads it.
	LinkedGuardians json.RawMessage `json:"-" swaggerignore:"true"`
}

// NewRiderGuardianDto is one guardian of the rider form: an account by user_id, a person to invite by
// email and name, or a contact with only a name and a phone (TRACK-037 D3).
type NewRiderGuardianDto struct {
	UserID    *uuid.UUID `json:"user_id" binding:"omitempty,uuidv4"`
	Email     *string    `json:"email" binding:"omitempty,email-valid,max=255" conform:"trim,lowercase"`
	FirstName *string    `json:"first_name" binding:"omitempty,max=100" conform:"trim"`
	LastName  *string    `json:"last_name" binding:"omitempty,max=100" conform:"trim"`
	Phone     *string    `json:"phone" binding:"omitempty,max=50" conform:"trim"`
}

// CreatedRider is the 201 of POST /tracking/riders: the rider and the accounts its guardians got.
type CreatedRider struct {
	*Rider
	Guardians []*AccessGranted `json:"guardians"`
}

// UpdateRiderDto represents request to update rider
type UpdateRiderDto struct {
	Phone                 *string  `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	Email                 *string  `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	EmergencyContactName  *string  `json:"emergency_contact_name" binding:"omitempty,max=255" conform:"trim"`
	EmergencyContactPhone *string  `json:"emergency_contact_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianName          *string  `json:"guardian_name" binding:"omitempty,max=255" conform:"trim"`
	GuardianPhone         *string  `json:"guardian_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianEmail         *string  `json:"guardian_email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Address               *string  `json:"address" binding:"omitempty,max=500" conform:"trim"`
	HomeLatitude          *float64 `json:"home_latitude" binding:"required_with=HomeLongitude,omitempty,min=-90,max=90"`
	HomeLongitude         *float64 `json:"home_longitude" binding:"required_with=HomeLatitude,omitempty,min=-180,max=180"`
	Notes                 *string  `json:"notes" binding:"omitempty,max=2000" conform:"trim"`
	// GroupLabel: nil keeps the group, "" clears it (TRACK-039 D2).
	GroupLabel *string `json:"group_label" binding:"omitempty,max=100" conform:"trim"`
	IsActive   *bool   `json:"is_active"`
}

// === DTOs for Route Management ===

// ListRoutesQuery binds GET /tracking/routes (TRACK-002 D2). Every filter is optional; search matches
// the name or the code.
type ListRoutesQuery struct {
	Search    string  `form:"search"`
	CompanyID *string `form:"company_id" binding:"omitempty,uuid"`
	// VehicleID narrows to the routes one vehicle runs: the vehicle's Routes tab (TRACK-034).
	VehicleID *string `form:"vehicle_id" binding:"omitempty,uuid"`
	// OrganizationID narrows to one destination's routes (TRACK-038).
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
	Direction      string  `form:"direction" binding:"omitempty,oneof=outbound inbound"`
	IsActive       *bool   `form:"is_active"`
	Page           int     `form:"page,default=1" binding:"min=1"`
	PageSize       int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListRoutesResponse struct {
	Routes     []*Route `json:"routes"`
	TotalCount int64    `json:"total_count"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

// CreateRouteDto represents request to create route. Recurrence is the route's schedules
// (TRACK-018), not fields here (TRACK-002 D3).
type CreateRouteDto struct {
	CompanyID                uuid.UUID  `json:"company_id" binding:"required,uuidv4"`
	RouteName                string     `json:"route_name" binding:"required,min=3,max=255" conform:"trim"`
	OriginAddress            string     `json:"origin_address" binding:"required_without=OrganizationID,omitempty,min=5,max=500" conform:"trim"`
	DestinationAddress       string     `json:"destination_address" binding:"required_without=OrganizationID,omitempty,min=5,max=500" conform:"trim"`
	Direction                *string    `json:"direction" binding:"omitempty,oneof=outbound inbound"`
	RouteCode                *string    `json:"route_code" binding:"omitempty,max=50" conform:"trim,uppercase"`
	VehicleID                *uuid.UUID `json:"vehicle_id" binding:"omitempty,uuidv4"`
	DefaultDriverID          *uuid.UUID `json:"default_driver_id" binding:"omitempty,uuidv4"`
	Timezone                 *string    `json:"timezone" binding:"omitempty,max=64"`
	OriginLat                *float64   `json:"origin_lat" binding:"omitempty,min=-90,max=90"`
	OriginLng                *float64   `json:"origin_lng" binding:"omitempty,min=-180,max=180"`
	DestinationLat           *float64   `json:"destination_lat" binding:"omitempty,min=-90,max=90"`
	DestinationLng           *float64   `json:"destination_lng" binding:"omitempty,min=-180,max=180"`
	EstimatedDurationMinutes *int32     `json:"estimated_duration_minutes" binding:"omitempty,min=1,max=999"`
	// Capacity overrides the vehicle's seats for this route (TRACK-023 D2); 0 clears the override.
	Capacity *int32 `json:"capacity" binding:"omitempty,min=0,max=200"`

	// A destination route (TRACK-038): with organization_id the route ends at that school or company
	// and, with with_return (the default), its return is created beside it. Origin and destination
	// come from the stops and the organization, so neither address is asked.
	OrganizationID  *uuid.UUID       `json:"organization_id" binding:"omitempty,uuidv4"`
	WithReturn      *bool            `json:"with_return"`
	ArrivalTime     *string          `json:"arrival_time" binding:"omitempty,datetime=15:04"`
	DepartureTime   *string          `json:"departure_time" binding:"omitempty,datetime=15:04"`
	DaysOfWeek      *int16           `json:"days_of_week" binding:"omitempty,min=1,max=127"`
	Stops           []map[string]any `json:"stops" binding:"omitempty,max=100"`
	ReturnRouteName *string          `json:"return_route_name" binding:"omitempty,min=3,max=255" conform:"trim"`
}

// CreatedRoute is the 201 of POST /tracking/routes: the route, and its return when one was created.
type CreatedRoute struct {
	*Route
	ReturnRoute *Route `json:"return_route"`
}

// UpdateRouteDto represents request to update route. company_id is immutable. VehicleID and
// DefaultDriverID are written as sent — omitting them clears them — because the form owns both;
// every other field keeps its value when omitted, and an empty route_code clears the code.
type UpdateRouteDto struct {
	RouteName                *string    `json:"route_name" binding:"omitempty,min=3,max=255" conform:"trim"`
	RouteCode                *string    `json:"route_code" binding:"omitempty,max=50" conform:"trim,uppercase"`
	Direction                *string    `json:"direction" binding:"omitempty,oneof=outbound inbound"`
	VehicleID                *uuid.UUID `json:"vehicle_id" binding:"omitempty,uuidv4"`
	DefaultDriverID          *uuid.UUID `json:"default_driver_id" binding:"omitempty,uuidv4"`
	OriginAddress            *string    `json:"origin_address" binding:"omitempty,min=5,max=500" conform:"trim"`
	OriginLat                *float64   `json:"origin_lat" binding:"omitempty,min=-90,max=90"`
	OriginLng                *float64   `json:"origin_lng" binding:"omitempty,min=-180,max=180"`
	DestinationAddress       *string    `json:"destination_address" binding:"omitempty,min=5,max=500" conform:"trim"`
	DestinationLat           *float64   `json:"destination_lat" binding:"omitempty,min=-90,max=90"`
	DestinationLng           *float64   `json:"destination_lng" binding:"omitempty,min=-180,max=180"`
	EstimatedDurationMinutes *int32     `json:"estimated_duration_minutes" binding:"omitempty,min=1,max=999"`
	// Capacity overrides the vehicle's seats for this route (TRACK-023 D2); 0 clears the override.
	Capacity *int32 `json:"capacity" binding:"omitempty,min=0,max=200"`
	IsActive *bool  `json:"is_active"`
	// Timezone is the IANA zone the route runs in (TRACK-008 D2); the SP refuses a name PostgreSQL
	// does not know.
	Timezone *string `json:"timezone" binding:"omitempty,max=64"`
}

// === DTOs for Route Versions (TRACK-007) ===

// RouteVersionStopDto is one line of the editor's list. Sequence is a sort key, not the stored
// number: the SP renumbers the list 1..n in the order asked for, so a reorder is one request and
// cannot collide with itself halfway through.
//
// A line without StopPlaceID is a place added from the route's map (TRACK-034 D3): the SP reuses the
// tenant's place within 30 m of it, or creates it, in the same statement that writes the list.
type RouteVersionStopDto struct {
	StopPlaceID      *uuid.UUID `json:"stop_place_id,omitempty" binding:"omitempty,uuidv4"`
	Name             *string    `json:"name,omitempty" binding:"required_without=StopPlaceID,omitempty,max=255"`
	Address          *string    `json:"address,omitempty" binding:"omitempty,max=500"`
	Latitude         *float64   `json:"latitude,omitempty" binding:"required_without=StopPlaceID,omitempty,min=-90,max=90"`
	Longitude        *float64   `json:"longitude,omitempty" binding:"required_without=StopPlaceID,omitempty,min=-180,max=180"`
	Sequence         int32      `json:"sequence" binding:"required,min=1"`
	PlannedOffsetMin *int32     `json:"planned_offset_min" binding:"omitempty,min=0,max=1440"`
	DwellSec         *int32     `json:"dwell_sec" binding:"omitempty,min=0,max=3600"`
}

// SetVehicleDriverDto sets who usually drives a vehicle (TRACK-034 D1); a null DriverID clears it.
type SetVehicleDriverDto struct {
	DriverID *uuid.UUID `json:"driver_id" binding:"omitempty,uuidv4"`
}

// VehicleConflictsQuery asks which runs would overlap: RouteID being assigned to the vehicle, or —
// without it — the vehicle's own routes, with DriverID about to drive them (TRACK-034 D2).
type VehicleConflictsQuery struct {
	RouteID  *string `form:"route_id" binding:"omitempty,uuid"`
	DriverID *string `form:"driver_id" binding:"omitempty,uuid"`
}

type VehicleConflictsResponse struct {
	Conflicts []*VehicleScheduleConflict `json:"conflicts"`
	// SeatsShort are the routes the vehicle would carry with more riders than seats (TRACK-038 D3).
	SeatsShort []*VehicleSeatsShort `json:"seats_short"`
}

// CreateRouteVersionDto publishes a route's next stop list. The version in force is closed the day
// before EffectiveFrom, so the two cover the calendar without a gap.
type CreateRouteVersionDto struct {
	EffectiveFrom coreModels.DateOnly   `json:"effective_from" binding:"required" time_format:"2006-01-02"`
	Stops         []RouteVersionStopDto `json:"stops" binding:"omitempty,dive"`
}

// ReplaceRouteVersionStopsDto is the body of PUT …/versions/:vid/stops — the whole list, every time.
type ReplaceRouteVersionStopsDto struct {
	Stops []RouteVersionStopDto `json:"stops" binding:"omitempty,dive"`
}

// === DTOs for Schedules and Calendars (TRACK-018) ===

// CreateRouteScheduleDto adds a recurrence to a route. DaysOfWeek is the bitmask of D1 — bit 0
// Monday … bit 6 Sunday — so 0 would be a schedule that never runs and is refused here rather than
// stored. StartTime crosses the wire as HH:MM, which is what comes back.
type CreateRouteScheduleDto struct {
	DaysOfWeek int16                `json:"days_of_week" binding:"required,min=1,max=127"`
	StartTime  string               `json:"start_time" binding:"required,datetime=15:04"`
	ValidFrom  coreModels.DateOnly  `json:"valid_from" binding:"required" time_format:"2006-01-02"`
	ValidUntil *coreModels.DateOnly `json:"valid_until" binding:"omitempty" time_format:"2006-01-02"`
	CalendarID *uuid.UUID           `json:"calendar_id" binding:"omitempty,uuidv4"`
}

// UpdateRouteScheduleDto edits a schedule in place. Every field is optional: the SP keeps what it is
// not sent. Moving the validity of a schedule that is already running is what a split is for — this
// is for fixing a schedule that was entered wrong. A null cannot say "remove", so the two clear flags
// do: they take the end date or the calendar away and win over the matching field (TRACK-029 D4).
type UpdateRouteScheduleDto struct {
	DaysOfWeek      *int16               `json:"days_of_week" binding:"omitempty,min=1,max=127"`
	StartTime       *string              `json:"start_time" binding:"omitempty,datetime=15:04"`
	ValidFrom       *coreModels.DateOnly `json:"valid_from" binding:"omitempty" time_format:"2006-01-02"`
	ValidUntil      *coreModels.DateOnly `json:"valid_until" binding:"omitempty" time_format:"2006-01-02"`
	CalendarID      *uuid.UUID           `json:"calendar_id" binding:"omitempty,uuidv4"`
	ClearValidUntil bool                 `json:"clear_valid_until"`
	ClearCalendar   bool                 `json:"clear_calendar"`
}

// RouteScheduleChangesDto is what the new half of a split carries. A field left out keeps what the
// closed half had, which is why every field is a pointer and every json tag omits an empty one: the
// SP reads the object by key presence.
type RouteScheduleChangesDto struct {
	DaysOfWeek *int16     `json:"days_of_week,omitempty" binding:"omitempty,min=1,max=127"`
	StartTime  *string    `json:"start_time,omitempty" binding:"omitempty,datetime=15:04"`
	CalendarID *uuid.UUID `json:"calendar_id,omitempty" binding:"omitempty,uuidv4"`
}

// SplitRouteScheduleDto is "from this date onward": the schedule in force is closed the day before
// FromDate and a new one carrying Changes opens on it.
type SplitRouteScheduleDto struct {
	FromDate coreModels.DateOnly     `json:"from_date" binding:"required" time_format:"2006-01-02"`
	Changes  RouteScheduleChangesDto `json:"changes"`
}

// SplitRouteScheduleResponse is both halves of a split, so the editor can show what it did without
// re-reading the list.
type SplitRouteScheduleResponse struct {
	Closed  *RouteSchedule `json:"closed"`
	Created *RouteSchedule `json:"created"`
}

// CreateCalendarDto represents request to create a calendar.
type CreateCalendarDto struct {
	Name           string     `json:"name" binding:"required,min=2,max=255" conform:"trim"`
	OrganizationID *uuid.UUID `json:"organization_id" binding:"omitempty,uuidv4"`
}

// UpdateCalendarDto represents request to edit a calendar. Every field is optional: the SP keeps
// what it is not sent.
type UpdateCalendarDto struct {
	Name           *string    `json:"name" binding:"omitempty,min=2,max=255" conform:"trim"`
	OrganizationID *uuid.UUID `json:"organization_id" binding:"omitempty,uuidv4"`
}

// ListCalendarsQuery binds GET /tracking/calendars.
type ListCalendarsQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListCalendarsResponse struct {
	Calendars  []*Calendar `json:"calendars"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

// CalendarDateDto is one line of the calendar editor's list. The whole list travels in one PUT, so
// a school year is saved once rather than one holiday at a time.
type CalendarDateDto struct {
	Date  coreModels.DateOnly `json:"date" binding:"required" time_format:"2006-01-02"`
	Kind  string              `json:"kind" binding:"required,oneof=no_service special_service"`
	Label *string             `json:"label" binding:"omitempty,max=255" conform:"trim"`
}

// ListCalendarDatesQuery binds GET /tracking/calendars/:id/dates. Both bounds are optional and an
// absent one means "no bound on that side".
type ListCalendarDatesQuery struct {
	From *string `form:"from"`
	To   *string `form:"to"`
}

// === DTOs for Exceptions and Preview (TRACK-018) ===

// CreateRouteExceptionDto records what happens differently over a range of dates. DateFrom equals
// DateTo for the common case, a single day off. Payload stays an untyped object because its shape
// is the kind's contract and the SP is where that contract is checked — seven typed structs here
// would be seven places to keep in step with it.
type CreateRouteExceptionDto struct {
	DateFrom coreModels.DateOnly    `json:"date_from" binding:"required" time_format:"2006-01-02"`
	DateTo   coreModels.DateOnly    `json:"date_to" binding:"required" time_format:"2006-01-02"`
	Kind     string                 `json:"kind" binding:"required,oneof=cancel change_time change_vehicle change_driver skip_stop add_stop detour"`
	Payload  map[string]interface{} `json:"payload"`
	Reason   *string                `json:"reason" binding:"omitempty,max=500" conform:"trim"`
}

// ListRouteExceptionsQuery binds GET /tracking/routes/:route_id/exceptions. An exception that starts
// before the window and runs into it is part of the answer, so the filter is an overlap.
type ListRouteExceptionsQuery struct {
	DateFrom *string `form:"date_from"`
	DateTo   *string `form:"date_to"`
}

// RoutePreviewQuery binds GET /tracking/routes/:route_id/preview. Both bounds are required: an open
// window is one generate_series away from a request that scans years.
type RoutePreviewQuery struct {
	From string `form:"from" binding:"required,datetime=2006-01-02"`
	To   string `form:"to" binding:"required,datetime=2006-01-02"`
}

// === DTOs for Stop Places (TRACK-007) ===

// CreateStopPlaceDto represents request to create a stop place. Latitude and longitude are pointers
// so that 0 — the equator and the prime meridian — is a coordinate and not an absent field.
type CreateStopPlaceDto struct {
	Name           string     `json:"name" binding:"required,min=2,max=255" conform:"trim"`
	Address        *string    `json:"address" binding:"omitempty,max=500" conform:"trim"`
	Latitude       *float64   `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude      *float64   `json:"longitude" binding:"required,min=-180,max=180"`
	OrganizationID *uuid.UUID `json:"organization_id" binding:"omitempty,uuidv4"`
}

// UpdateStopPlaceDto represents request to edit a stop place. Every field is optional: the SP keeps
// what it is not sent, and the coordinate moves only when both halves arrive.
type UpdateStopPlaceDto struct {
	Name           *string    `json:"name" binding:"omitempty,min=2,max=255" conform:"trim"`
	Address        *string    `json:"address" binding:"omitempty,max=500" conform:"trim"`
	Latitude       *float64   `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude      *float64   `json:"longitude" binding:"omitempty,min=-180,max=180"`
	OrganizationID *uuid.UUID `json:"organization_id" binding:"omitempty,uuidv4"`
}

// ListStopPlacesQuery binds GET /tracking/stop-places. `near=lat,lng` turns the list into the stop
// places within `radius_m` of that point, nearest first, each carrying distance_m.
type ListStopPlacesQuery struct {
	Search   string  `form:"search"`
	Near     *string `form:"near"`
	RadiusM  *int32  `form:"radius_m" binding:"omitempty,min=1,max=100000"`
	Page     int     `form:"page,default=1" binding:"min=1"`
	PageSize int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ErrInvalidNear is what NearPoint returns for a `near` the handler cannot read as a coordinate.
var ErrInvalidNear = errors.New("near must be `latitude,longitude` within (-90..90, -180..180)")

// NearPoint reads the `near` parameter. It answers (nil, nil, nil) when the caller did not send one,
// so the same call covers both shapes of the list.
func (q ListStopPlacesQuery) NearPoint() (latitude, longitude *float64, err error) {
	if q.Near == nil || strings.TrimSpace(*q.Near) == "" {
		return nil, nil, nil
	}
	parts := strings.Split(*q.Near, ",")
	if len(parts) != 2 {
		return nil, nil, ErrInvalidNear
	}
	lat, latErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, lngErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if latErr != nil || lngErr != nil || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, nil, ErrInvalidNear
	}
	return &lat, &lng, nil
}

type ListStopPlacesResponse struct {
	StopPlaces []*StopPlace `json:"stop_places"`
	TotalCount int64        `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

// === DTOs for Route Stop Management ===

// ListRouteStopsQuery binds GET /tracking/routes/:route_id/stops (TRACK-007 D1). Date picks the day
// whose list is wanted; unset means today, which is what every caller that does not care about
// history sends.
type ListRouteStopsQuery struct {
	Date *string `form:"date" binding:"omitempty,datetime=2006-01-02"`
}

// === DTOs for Rider Assignment ===

// AssignRiderDto is one assignment to create — the body of POST /tracking/assignments and one item
// of the bulk list. It travels to the SP as JSON, so a stop or an end date left out is a key left
// out. DaysOfWeek is the bitmask of RouteSchedule.
type AssignRiderDto struct {
	RiderID            uuid.UUID            `json:"rider_id" binding:"required,uuidv4"`
	RouteID            uuid.UUID            `json:"route_id" binding:"required,uuidv4"`
	DaysOfWeek         int16                `json:"days_of_week" binding:"required,min=1,max=127"`
	PickupStopPlaceID  *uuid.UUID           `json:"pickup_stop_place_id,omitempty" binding:"omitempty,uuidv4"`
	DropoffStopPlaceID *uuid.UUID           `json:"dropoff_stop_place_id,omitempty" binding:"omitempty,uuidv4"`
	ValidFrom          coreModels.DateOnly  `json:"valid_from" binding:"required" time_format:"2006-01-02"`
	ValidUntil         *coreModels.DateOnly `json:"valid_until,omitempty" binding:"omitempty" time_format:"2006-01-02"`
	// Legs on a paired route (TRACK-038 D1): both (the default) also writes the twin on the other route.
	Legs *string `json:"legs,omitempty" binding:"omitempty,oneof=both outbound return"`
}

// UpdateAssignmentDto edits an assignment in place. Every field is optional and a field left out
// keeps its value — the SP reads the object by key presence, hence omitempty everywhere. The rider
// is not editable: another rider is a delete and a create.
type UpdateAssignmentDto struct {
	RouteID            *uuid.UUID           `json:"route_id,omitempty" binding:"omitempty,uuidv4"`
	DaysOfWeek         *int16               `json:"days_of_week,omitempty" binding:"omitempty,min=1,max=127"`
	PickupStopPlaceID  *uuid.UUID           `json:"pickup_stop_place_id,omitempty" binding:"omitempty,uuidv4"`
	DropoffStopPlaceID *uuid.UUID           `json:"dropoff_stop_place_id,omitempty" binding:"omitempty,uuidv4"`
	ValidFrom          *coreModels.DateOnly `json:"valid_from,omitempty" binding:"omitempty" time_format:"2006-01-02"`
	ValidUntil         *coreModels.DateOnly `json:"valid_until,omitempty" binding:"omitempty" time_format:"2006-01-02"`
}

// ListAssignmentsQuery binds GET /tracking/assignments. Date narrows to the assignments in force
// that day. A malformed id or date is a 400, not a silently unfiltered list.
type ListAssignmentsQuery struct {
	RiderID  *string `form:"rider_id" binding:"omitempty,uuid"`
	RouteID  *string `form:"route_id" binding:"omitempty,uuid"`
	Date     *string `form:"date" binding:"omitempty,datetime=2006-01-02"`
	Page     int     `form:"page,default=1" binding:"min=1"`
	PageSize int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListAssignmentsResponse struct {
	Assignments []*RiderRouteAssignment `json:"assignments"`
	TotalCount  int64                   `json:"total_count"`
	Page        int                     `json:"page"`
	PageSize    int                     `json:"page_size"`
}

// AssignmentResponse is what a single create or an edit answers: the row and the capacity warnings
// of its route.
type AssignmentResponse struct {
	Assignment *RiderRouteAssignment `json:"assignment"`
	Warnings   []AssignmentWarning   `json:"warnings"`
}

// BulkAssignmentResponse is what POST /tracking/assignments/bulk answers, in the order sent.
type BulkAssignmentResponse struct {
	Assignments []*RiderRouteAssignment `json:"assignments"`
	Warnings    []AssignmentWarning     `json:"warnings"`
}

// === DTOs for Organization Management ===

// CreateOrganizationDto represents request to create an organization (TRACK-005).
type CreateOrganizationDto struct {
	Name     string `json:"name" binding:"required,min=2,max=255" conform:"trim"`
	Kind     string `json:"kind" binding:"required,oneof=school company other" conform:"trim,lowercase"`
	Timezone string `json:"timezone" binding:"omitempty,max=64" conform:"trim"`
	IsActive *bool  `json:"is_active"`
	// Minutes before a pickup up to which a guardian may report an absence (TRACK-022 D1); 60 when unset.
	AbsenceCutoffMin *int32 `json:"absence_cutoff_min" binding:"omitempty,min=0,max=1440"`
	// The destination its routes take (TRACK-038 D2): both halves of the pin, or neither.
	Latitude  *float64 `json:"latitude" binding:"required_with=Longitude,omitempty,min=-90,max=90"`
	Longitude *float64 `json:"longitude" binding:"required_with=Latitude,omitempty,min=-180,max=180"`
	Address   *string  `json:"address" binding:"omitempty,max=500" conform:"trim"`
}

// UpdateOrganizationDto represents request to update an organization. Every field is optional: the
// SP keeps what it is not sent.
type UpdateOrganizationDto struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=255" conform:"trim"`
	Kind     *string `json:"kind" binding:"omitempty,oneof=school company other" conform:"trim,lowercase"`
	Timezone *string `json:"timezone" binding:"omitempty,max=64" conform:"trim"`
	IsActive *bool   `json:"is_active"`
	// Minutes before a pickup up to which a guardian may report an absence (TRACK-022 D1).
	AbsenceCutoffMin *int32   `json:"absence_cutoff_min" binding:"omitempty,min=0,max=1440"`
	Latitude         *float64 `json:"latitude" binding:"required_with=Longitude,omitempty,min=-90,max=90"`
	Longitude        *float64 `json:"longitude" binding:"required_with=Latitude,omitempty,min=-180,max=180"`
	Address          *string  `json:"address" binding:"omitempty,max=500" conform:"trim"`
}

// ListOrganizationsQuery binds GET /tracking/organizations.
type ListOrganizationsQuery struct {
	Search   string `form:"search"`
	Kind     string `form:"kind" binding:"omitempty,oneof=school company other"`
	IsActive *bool  `form:"is_active"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListOrganizationsResponse struct {
	Organizations []*Organization `json:"organizations"`
	TotalCount    int64           `json:"total_count"`
	Page          int             `json:"page"`
	PageSize      int             `json:"page_size"`
}

// UpsertOrganizationMemberDto is the body of PUT /tracking/organizations/:id/members/:user_id — the
// same call adds a member and changes the role of one already there.
type UpsertOrganizationMemberDto struct {
	Role string `json:"role" binding:"required,oneof=admin viewer supervisor" conform:"trim,lowercase"`
}

// === DTOs for Driver Management (TRACK-006) ===

// CreateDriverDto represents request to create a driver. The carrier is required and, once set,
// never changes — a driver moves carrier by being re-created, so the routes that name them as
// default driver are dealt with deliberately rather than re-scoped behind the operator's back.
type CreateDriverDto struct {
	CompanyID        uuid.UUID            `json:"company_id" binding:"required,uuidv4"`
	FirstName        string               `json:"first_name" binding:"required,min=2,max=255" conform:"trim"`
	LastName         string               `json:"last_name" binding:"required,min=2,max=255" conform:"trim"`
	Phone            *string              `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	LicenseNumber    string               `json:"license_number" binding:"required,min=2,max=100" conform:"trim"`
	LicenseClass     *string              `json:"license_class" binding:"omitempty,max=50" conform:"trim"`
	LicenseExpiresOn *coreModels.DateOnly `json:"license_expires_on" time_format:"2006-01-02"`
	Status           string               `json:"status" binding:"omitempty,oneof=active inactive suspended" conform:"trim,lowercase"`
}

// UpdateDriverDto represents request to update a driver. Every field is optional: the SP keeps what
// it is not sent. The account is not here: it is linked through PUT /drivers/:id/account, which
// holds it to the driver level (TRACK-032).
type UpdateDriverDto struct {
	FirstName        *string              `json:"first_name" binding:"omitempty,min=2,max=255" conform:"trim"`
	LastName         *string              `json:"last_name" binding:"omitempty,min=2,max=255" conform:"trim"`
	Phone            *string              `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	LicenseNumber    *string              `json:"license_number" binding:"omitempty,min=2,max=100" conform:"trim"`
	LicenseClass     *string              `json:"license_class" binding:"omitempty,max=50" conform:"trim"`
	LicenseExpiresOn *coreModels.DateOnly `json:"license_expires_on" time_format:"2006-01-02"`
	Status           *string              `json:"status" binding:"omitempty,oneof=active inactive suspended" conform:"trim,lowercase"`
}

// ListDriversQuery binds GET /tracking/drivers. search matches a name or the licence number.
type ListDriversQuery struct {
	Search    string  `form:"search"`
	CompanyID *string `form:"company_id" binding:"omitempty,uuid"`
	Status    string  `form:"status" binding:"omitempty,oneof=active inactive suspended"`
	Page      int     `form:"page,default=1" binding:"min=1"`
	PageSize  int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListDriversResponse struct {
	Drivers    []*Driver `json:"drivers"`
	TotalCount int64     `json:"total_count"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

// === DTOs for Compliance Documents (TRACK-016) ===

// CreateDocumentTypeDto represents request to add a line to the tenant's compliance policy.
type CreateDocumentTypeDto struct {
	Code           string `json:"code" binding:"required,min=2,max=50" conform:"trim,upper"`
	Name           string `json:"name" binding:"required,min=2,max=255" conform:"trim"`
	AppliesTo      string `json:"applies_to" binding:"required,oneof=vehicle driver both" conform:"trim,lowercase"`
	WarnDaysBefore *int32 `json:"warn_days_before" binding:"omitempty,min=0,max=365"`
	BlocksService  *bool  `json:"blocks_service"`
	IsActive       *bool  `json:"is_active"`
}

// UpdateDocumentTypeDto represents request to edit a policy line. Every field is optional: the SP
// keeps what it is not sent, and every document references its type by id, so nothing here is frozen.
type UpdateDocumentTypeDto struct {
	Code           *string `json:"code" binding:"omitempty,min=2,max=50" conform:"trim,upper"`
	Name           *string `json:"name" binding:"omitempty,min=2,max=255" conform:"trim"`
	AppliesTo      *string `json:"applies_to" binding:"omitempty,oneof=vehicle driver both" conform:"trim,lowercase"`
	WarnDaysBefore *int32  `json:"warn_days_before" binding:"omitempty,min=0,max=365"`
	BlocksService  *bool   `json:"blocks_service"`
	IsActive       *bool   `json:"is_active"`
}

// ListDocumentTypesQuery binds GET /tracking/document-types. The whole policy is a handful of rows,
// so there is no paging here.
type ListDocumentTypesQuery struct {
	AppliesTo string `form:"applies_to" binding:"omitempty,oneof=vehicle driver both"`
	IsActive  *bool  `form:"is_active"`
}

// CreateDocumentDto represents request to file a document against one vehicle or driver.
type CreateDocumentDto struct {
	SubjectType    string               `json:"subject_type" binding:"required,oneof=vehicle driver" conform:"trim,lowercase"`
	SubjectID      uuid.UUID            `json:"subject_id" binding:"required,uuidv4"`
	DocumentTypeID uuid.UUID            `json:"document_type_id" binding:"required,uuidv4"`
	Number         *string              `json:"number" binding:"omitempty,max=100" conform:"trim"`
	IssuedOn       *coreModels.DateOnly `json:"issued_on" time_format:"2006-01-02"`
	ExpiresOn      *coreModels.DateOnly `json:"expires_on" time_format:"2006-01-02"`
	Notes          *string              `json:"notes" binding:"omitempty,max=2000" conform:"trim"`
}

// UpdateDocumentDto represents request to edit a document. The subject and the type are set once: a
// document that changes either is a different document, and re-filing it keeps the history honest.
type UpdateDocumentDto struct {
	Number    *string              `json:"number" binding:"omitempty,max=100" conform:"trim"`
	IssuedOn  *coreModels.DateOnly `json:"issued_on" time_format:"2006-01-02"`
	ExpiresOn *coreModels.DateOnly `json:"expires_on" time_format:"2006-01-02"`
	Notes     *string              `json:"notes" binding:"omitempty,max=2000" conform:"trim"`
}

// ClaimDocumentFileDto binds PUT /tracking/documents/:id/file (INFRA-007 D1): the key a ticket from
// POST /storage/uploads issued, and what the client called its file — shown, never part of a key.
type ClaimDocumentFileDto struct {
	Key      string `json:"key" binding:"required,max=200"`
	Filename string `json:"filename" binding:"max=1000"`
}

// DocumentFileLink is a signed link to a document's file, good for a minute (INFRA-007 D2).
type DocumentFileLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ListDocumentsQuery binds GET /tracking/documents. ExpiringWithinDays narrows to what needs acting
// on; a document with no expiry is never part of that answer.
type ListDocumentsQuery struct {
	SubjectType        string  `form:"subject_type" binding:"omitempty,oneof=vehicle driver"`
	SubjectID          *string `form:"subject_id" binding:"omitempty,uuid"`
	ExpiringWithinDays *int32  `form:"expiring_within_days" binding:"omitempty,min=0,max=365"`
	Page               int     `form:"page,default=1" binding:"min=1"`
	PageSize           int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListDocumentsResponse struct {
	Documents  []*ComplianceDocument `json:"documents"`
	TotalCount int64                 `json:"total_count"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
}
