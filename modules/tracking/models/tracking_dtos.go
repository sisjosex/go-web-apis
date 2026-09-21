package models

import (
	"errors"
	"strconv"
	"strings"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// === DTOs for Location Updates ===

// UpdateLocationDto represents GPS location update from vehicle
type UpdateLocationDto struct {
	VehicleID  uuid.UUID `json:"vehicle_id" binding:"required,uuidv4"`
	Latitude   float64   `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude  float64   `json:"longitude" binding:"required,min=-180,max=180"`
	Speed      *float64  `json:"speed" binding:"omitempty,min=0"`
	Heading    *float64  `json:"heading" binding:"omitempty,min=0,max=360"`
	Altitude   *float64  `json:"altitude"`
	Accuracy   *float64  `json:"accuracy" binding:"omitempty,min=0"`
	RecordedAt *string   `json:"recorded_at"` // ISO8601 timestamp
}

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

// RecordEventDto represents a check-in/checkout event
type RecordEventDto struct {
	RiderID   uuid.UUID  `json:"rider_id" binding:"required,uuidv4"`
	RouteID   uuid.UUID  `json:"route_id" binding:"required,uuidv4"`
	VehicleID uuid.UUID  `json:"vehicle_id" binding:"required,uuidv4"`
	EventType string     `json:"event_type" binding:"required,oneof=check_in checkout no_show emergency" enums:"check_in,checkout,no_show,emergency"`
	StopID    *uuid.UUID `json:"stop_id" binding:"omitempty,uuidv4"`
	Latitude  *float64   `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude *float64   `json:"longitude" binding:"omitempty,min=-180,max=180"`
	Notes     *string    `json:"notes" binding:"omitempty,max=500"`
}

// EventRecordedResponse represents response after recording event
type EventRecordedResponse struct {
	EventID   uuid.UUID `json:"event_id"`
	RiderID   uuid.UUID `json:"rider_id"`
	RiderName string    `json:"rider_name"`
	EventType string    `json:"event_type"`
	EventTime string    `json:"event_time"`
	StopName  *string   `json:"stop_name"`
}

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
}

type ListRidersResponse struct {
	Riders     []*Rider `json:"riders"`
	TotalCount int64    `json:"total_count"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
}

// CreateRiderDto represents request to create rider
type CreateRiderDto struct {
	OrganizationID        uuid.UUID  `json:"organization_id" binding:"required,uuidv4"`
	RiderType             string     `json:"rider_type" binding:"required,oneof=student employee"`
	FirstName             string     `json:"first_name" binding:"required,min=2,max=255" conform:"trim"`
	LastName              string     `json:"last_name" binding:"required,min=2,max=255" conform:"trim"`
	IdentificationNumber  *string    `json:"identification_number" binding:"omitempty,max=100" conform:"trim"`
	Phone                 *string    `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	Email                 *string    `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	EmergencyContactName  *string    `json:"emergency_contact_name" binding:"omitempty,max=255" conform:"trim"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianUserID        *uuid.UUID `json:"guardian_user_id" binding:"omitempty,uuidv4"`
	GuardianName          *string    `json:"guardian_name" binding:"omitempty,max=255" conform:"trim"`
	GuardianPhone         *string    `json:"guardian_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianEmail         *string    `json:"guardian_email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Address               *string    `json:"address" binding:"omitempty,max=500" conform:"trim"`
}

// UpdateRiderDto represents request to update rider
type UpdateRiderDto struct {
	Phone                 *string    `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	Email                 *string    `json:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	EmergencyContactName  *string    `json:"emergency_contact_name" binding:"omitempty,max=255" conform:"trim"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianUserID        *uuid.UUID `json:"guardian_user_id" binding:"omitempty,uuidv4"`
	GuardianName          *string    `json:"guardian_name" binding:"omitempty,max=255" conform:"trim"`
	GuardianPhone         *string    `json:"guardian_phone" binding:"omitempty,max=50" conform:"trim"`
	GuardianEmail         *string    `json:"guardian_email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Address               *string    `json:"address" binding:"omitempty,max=500" conform:"trim"`
	IsActive              *bool      `json:"is_active"`
}

// === DTOs for Route Management ===

// CreateRouteDto represents request to create route
type CreateRouteDto struct {
	CompanyID                uuid.UUID  `json:"company_id" binding:"required,uuidv4"`
	RouteName                string     `json:"route_name" binding:"required,min=3,max=255" conform:"trim"`
	OriginAddress            string     `json:"origin_address" binding:"required,min=5,max=500" conform:"trim"`
	DestinationAddress       string     `json:"destination_address" binding:"required,min=5,max=500" conform:"trim"`
	RouteCode                *string    `json:"route_code" binding:"omitempty,max=50" conform:"trim,uppercase"`
	VehicleID                *uuid.UUID `json:"vehicle_id" binding:"omitempty,uuidv4"`
	OriginLat                *float64   `json:"origin_lat" binding:"omitempty,min=-90,max=90"`
	OriginLng                *float64   `json:"origin_lng" binding:"omitempty,min=-180,max=180"`
	DestinationLat           *float64   `json:"destination_lat" binding:"omitempty,min=-90,max=90"`
	DestinationLng           *float64   `json:"destination_lng" binding:"omitempty,min=-180,max=180"`
	ScheduleType             string     `json:"schedule_type" binding:"omitempty,oneof=morning afternoon custom"`
	ScheduledStartTime       *string    `json:"scheduled_start_time" binding:"omitempty"` // HH:MM:SS
	ScheduledEndTime         *string    `json:"scheduled_end_time" binding:"omitempty"`   // HH:MM:SS
	EstimatedDurationMinutes *int32     `json:"estimated_duration_minutes" binding:"omitempty,min=1,max=999"`
}

// UpdateRouteDto represents request to update route
type UpdateRouteDto struct {
	RouteName                *string    `json:"route_name" binding:"omitempty,min=3,max=255" conform:"trim"`
	RouteCode                *string    `json:"route_code" binding:"omitempty,max=50" conform:"trim,uppercase"`
	VehicleID                *uuid.UUID `json:"vehicle_id" binding:"omitempty,uuidv4"`
	OriginAddress            *string    `json:"origin_address" binding:"omitempty,min=5,max=500" conform:"trim"`
	OriginLat                *float64   `json:"origin_lat" binding:"omitempty,min=-90,max=90"`
	OriginLng                *float64   `json:"origin_lng" binding:"omitempty,min=-180,max=180"`
	DestinationAddress       *string    `json:"destination_address" binding:"omitempty,min=5,max=500" conform:"trim"`
	DestinationLat           *float64   `json:"destination_lat" binding:"omitempty,min=-90,max=90"`
	DestinationLng           *float64   `json:"destination_lng" binding:"omitempty,min=-180,max=180"`
	ScheduledStartTime       *string    `json:"scheduled_start_time" binding:"omitempty"`
	ScheduledEndTime         *string    `json:"scheduled_end_time" binding:"omitempty"`
	EstimatedDurationMinutes *int32     `json:"estimated_duration_minutes" binding:"omitempty,min=1,max=999"`
	IsActive                 *bool      `json:"is_active"`
}

// === DTOs for Route Versions (TRACK-007) ===

// RouteVersionStopDto is one line of the editor's list. Sequence is a sort key, not the stored
// number: the SP renumbers the list 1..n in the order asked for, so a reorder is one request and
// cannot collide with itself halfway through.
type RouteVersionStopDto struct {
	StopPlaceID      uuid.UUID `json:"stop_place_id" binding:"required,uuidv4"`
	Sequence         int32     `json:"sequence" binding:"required,min=1"`
	PlannedOffsetMin *int32    `json:"planned_offset_min" binding:"omitempty,min=0,max=1440"`
	DwellSec         *int32    `json:"dwell_sec" binding:"omitempty,min=0,max=3600"`
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

// AssignRiderDto represents request to assign rider to route
type AssignRiderDto struct {
	RiderID       uuid.UUID  `json:"rider_id" binding:"required,uuidv4"`
	RouteID       uuid.UUID  `json:"route_id" binding:"required,uuidv4"`
	PickupStopID  *uuid.UUID `json:"pickup_stop_id" binding:"omitempty,uuidv4"`
	DropoffStopID *uuid.UUID `json:"dropoff_stop_id" binding:"omitempty,uuidv4"`
}

// === DTOs for Organization Management ===

// CreateOrganizationDto represents request to create an organization (TRACK-005).
type CreateOrganizationDto struct {
	Name     string `json:"name" binding:"required,min=2,max=255" conform:"trim"`
	Kind     string `json:"kind" binding:"required,oneof=school company other" conform:"trim,lowercase"`
	Timezone string `json:"timezone" binding:"required,max=64" conform:"trim"`
	IsActive *bool  `json:"is_active"`
}

// UpdateOrganizationDto represents request to update an organization. Every field is optional: the
// SP keeps what it is not sent.
type UpdateOrganizationDto struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=255" conform:"trim"`
	Kind     *string `json:"kind" binding:"omitempty,oneof=school company other" conform:"trim,lowercase"`
	Timezone *string `json:"timezone" binding:"omitempty,max=64" conform:"trim"`
	IsActive *bool   `json:"is_active"`
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
	UserID           *uuid.UUID           `json:"user_id" binding:"omitempty,uuidv4"`
	FirstName        string               `json:"first_name" binding:"required,min=2,max=255" conform:"trim"`
	LastName         string               `json:"last_name" binding:"required,min=2,max=255" conform:"trim"`
	Phone            *string              `json:"phone" binding:"omitempty,max=50" conform:"trim"`
	LicenseNumber    string               `json:"license_number" binding:"required,min=2,max=100" conform:"trim"`
	LicenseClass     *string              `json:"license_class" binding:"omitempty,max=50" conform:"trim"`
	LicenseExpiresOn *coreModels.DateOnly `json:"license_expires_on" time_format:"2006-01-02"`
	Status           string               `json:"status" binding:"omitempty,oneof=active inactive suspended" conform:"trim,lowercase"`
}

// UpdateDriverDto represents request to update a driver. Every field is optional: the SP keeps what
// it is not sent. ClearUserID is the one thing no value of UserID can say — "unlink the account" —
// and the driver screen's Unlink is what sends it.
type UpdateDriverDto struct {
	UserID           *uuid.UUID           `json:"user_id" binding:"omitempty,uuidv4"`
	ClearUserID      bool                 `json:"clear_user_id"`
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
