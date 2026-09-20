package models

import (
	"time"

	"github.com/google/uuid"
)

// TransportCompany represents a school, corporate, or transport provider
type TransportCompany struct {
	ID                 uuid.UUID `json:"id"`
	TenantID           uuid.UUID `json:"tenant_id"`
	Name               string    `json:"name"`
	Email              *string   `json:"email"`
	Phone              *string   `json:"phone"`
	Address            *string   `json:"address"`
	City               *string   `json:"city"`
	Country            *string   `json:"country"`
	RegistrationNumber string    `json:"registration_number"`
	Status             *string   `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Vehicle represents a bus or van with GPS tracking
type Vehicle struct {
	ID        uuid.UUID `json:"id"`
	CompanyID uuid.UUID `json:"company_id"`
	// CompanyName rides on the list rows only (TRACK-001 D2); the single-vehicle SPs do not
	// join the company, so it is absent from create/update/get responses.
	CompanyName *string   `json:"company_name,omitempty"`
	PlateNumber string    `json:"plate_number"`
	VehicleType string    `json:"vehicle_type"` // bus, van, car
	Brand       *string   `json:"brand"`
	Model       *string   `json:"model"`
	Year        *int32    `json:"year"`
	Capacity    int32     `json:"capacity"`
	GPSDeviceID *string   `json:"gps_device_id"`
	Status      string    `json:"status"` // active, inactive, maintenance
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Route represents a transport route with origin and destination
type Route struct {
	ID                       uuid.UUID  `json:"id"`
	CompanyID                uuid.UUID  `json:"company_id"`
	VehicleID                *uuid.UUID `json:"vehicle_id"`
	RouteName                string     `json:"route_name"`
	RouteCode                *string    `json:"route_code"`
	OriginAddress            string     `json:"origin_address"`
	OriginLat                *float64   `json:"origin_lat"`
	OriginLng                *float64   `json:"origin_lng"`
	DestinationAddress       string     `json:"destination_address"`
	DestinationLat           *float64   `json:"destination_lat"`
	DestinationLng           *float64   `json:"destination_lng"`
	ScheduleType             string     `json:"schedule_type"` // morning, afternoon, custom
	ScheduledStartTime       *string    `json:"scheduled_start_time"`
	ScheduledEndTime         *string    `json:"scheduled_end_time"`
	EstimatedDurationMinutes *int32     `json:"estimated_duration_minutes"`
	IsActive                 bool       `json:"is_active"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

// RouteStop represents an intermediate stop along a route
type RouteStop struct {
	ID                            uuid.UUID `json:"id"`
	RouteID                       uuid.UUID `json:"route_id"`
	StopName                      string    `json:"stop_name"`
	Address                       string    `json:"address"`
	Latitude                      *float64  `json:"latitude"`
	Longitude                     *float64  `json:"longitude"`
	StopOrder                     int32     `json:"stop_order"`
	ScheduledArrivalOffsetMinutes *int32    `json:"scheduled_arrival_offset_minutes"`
	IsActive                      bool      `json:"is_active"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

// Organization represents a school or employer whose people ride (TRACK-005). Kind is the only
// thing that tells one from another; timezone is an IANA name, read per organization so a pickup
// window means the same thing wherever the tenant operates.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Kind      string    `json:"kind"` // school, company, other
	Name      string    `json:"name"`
	Timezone  string    `json:"timezone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OrganizationMember is a tenant user on an organization, with the identity joined from auth.users.
// The three name fields are nullable: a member whose user row was deleted keeps its seat until an
// operator removes it (TRACK-005 D3).
type OrganizationMember struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     *string   `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
	Role      string    `json:"role"` // admin, viewer, supervisor
}

// Rider represents a student or employee using transport
type Rider struct {
	ID uuid.UUID `json:"id"`
	// The organization that sends the rider, not the carrier that drives them (TRACK-005 D1); the
	// carrier is reached through the route the rider is assigned to.
	OrganizationID        uuid.UUID  `json:"organization_id"`
	RiderType             *string    `json:"rider_type"` // student, employee (nullable as it's not persisted)
	FirstName             string     `json:"first_name"`
	LastName              string     `json:"last_name"`
	IdentificationNumber  *string    `json:"identification_number"`
	Phone                 *string    `json:"phone"`
	Email                 *string    `json:"email"`
	EmergencyContactName  *string    `json:"emergency_contact_name"`
	EmergencyContactPhone *string    `json:"emergency_contact_phone"`
	GuardianUserID        *uuid.UUID `json:"guardian_user_id"`
	GuardianName          *string    `json:"guardian_name"`
	GuardianPhone         *string    `json:"guardian_phone"`
	GuardianEmail         *string    `json:"guardian_email"`
	Address               *string    `json:"address"`
	IsActive              bool       `json:"is_active"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// RiderAssignment maps riders to routes with pickup/dropoff stops
type RiderAssignment struct {
	ID            uuid.UUID  `json:"id"`
	RiderID       uuid.UUID  `json:"rider_id"`
	RouteID       uuid.UUID  `json:"route_id"`
	PickupStopID  *uuid.UUID `json:"pickup_stop_id"`
	DropoffStopID *uuid.UUID `json:"dropoff_stop_id"`
	IsActive      bool       `json:"is_active"`
	AssignedAt    time.Time  `json:"assigned_at"`
	UnassignedAt  *time.Time `json:"unassigned_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// VehicleLocation represents GPS coordinates of a vehicle
type VehicleLocation struct {
	ID         int64     `json:"id"`
	VehicleID  uuid.UUID `json:"vehicle_id"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Speed      *float64  `json:"speed"`
	Heading    *float64  `json:"heading"`
	Altitude   *float64  `json:"altitude"`
	Accuracy   *float64  `json:"accuracy"`
	RecordedAt time.Time `json:"recorded_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// RideEvent represents boarding/arrival events
type RideEvent struct {
	ID           uuid.UUID  `json:"id"`
	RiderID      uuid.UUID  `json:"rider_id"`
	RouteID      uuid.UUID  `json:"route_id"`
	VehicleID    uuid.UUID  `json:"vehicle_id"`
	AssignmentID *uuid.UUID `json:"assignment_id"`
	EventType    string     `json:"event_type"` // check_in, checkout, no_show, emergency
	StopID       *uuid.UUID `json:"stop_id"`
	Latitude     *float64   `json:"latitude"`
	Longitude    *float64   `json:"longitude"`
	EventTime    time.Time  `json:"event_time"`
	Notes        *string    `json:"notes"`
	CreatedBy    *uuid.UUID `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
}

// RouteAlert represents route delays/incidents
type RouteAlert struct {
	ID                    uuid.UUID  `json:"id"`
	RouteID               uuid.UUID  `json:"route_id"`
	VehicleID             *uuid.UUID `json:"vehicle_id"`
	AlertType             string     `json:"alert_type"` // delay, breakdown, cancellation, emergency, other
	Severity              string     `json:"severity"`   // low, medium, high, critical
	Title                 string     `json:"title"`
	Message               string     `json:"message"`
	EstimatedDelayMinutes *int32     `json:"estimated_delay_minutes"`
	IsActive              bool       `json:"is_active"`
	CreatedBy             *uuid.UUID `json:"created_by"`
	CreatedAt             time.Time  `json:"created_at"`
	ResolvedAt            *time.Time `json:"resolved_at"`
	ResolvedBy            *uuid.UUID `json:"resolved_by"`
}
