package models

import (
	"encoding/json"
	"time"

	coreModels "josex/web/modules/core/models"

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
	// ServiceBlocked rides on the list rows only, like CompanyName: an expired document of a
	// blocks_service type stands against this vehicle (TRACK-016 D2).
	ServiceBlocked *bool `json:"service_blocked,omitempty"`
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

// StopPlace is a place a route calls at (TRACK-007 D1) — named once and shared by every route that
// stops there, rather than duplicated per route. OrganizationID is the school or employer whose gate
// this is, and NULL for a public corner. DistanceM rides only on the rows a `near` search returns.
type StopPlace struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Name           string     `json:"name"`
	Address        *string    `json:"address"`
	Latitude       *float64   `json:"latitude"`
	Longitude      *float64   `json:"longitude"`
	DistanceM      *float64   `json:"distance_m,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RouteVersion is one route's stop list over a range of dates (TRACK-007). EffectiveTo NULL is the
// version in force from EffectiveFrom onwards; publishing a new one closes it the day before.
type RouteVersion struct {
	ID            uuid.UUID            `json:"id"`
	RouteID       uuid.UUID            `json:"route_id"`
	EffectiveFrom coreModels.DateOnly  `json:"effective_from"`
	EffectiveTo   *coreModels.DateOnly `json:"effective_to"`
	StopsCount    int32                `json:"stops_count"`
	CreatedBy     *uuid.UUID           `json:"created_by"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// RouteStop is one stop on one version of a route: the place, where it sits in the order, and when
// the bus is due. ID is the row in the version's list; StopPlaceID is the place it names.
type RouteStop struct {
	ID               uuid.UUID `json:"id"`
	RouteID          uuid.UUID `json:"route_id"`
	VersionID        uuid.UUID `json:"version_id"`
	StopPlaceID      uuid.UUID `json:"stop_place_id"`
	StopName         string    `json:"stop_name"`
	Address          *string   `json:"address"`
	Latitude         *float64  `json:"latitude"`
	Longitude        *float64  `json:"longitude"`
	Sequence         int32     `json:"sequence"`
	PlannedOffsetMin *int32    `json:"planned_offset_min"`
	DwellSec         *int32    `json:"dwell_sec"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RouteSchedule is when a route runs (TRACK-018 D1): a weekday bitmask — bit 0 Monday … bit 6
// Sunday — and a departure time, over a range of dates. ValidUntil NULL is "and every day after".
// CalendarName rides along so a list needs no second request to say which holidays apply.
type RouteSchedule struct {
	ID           uuid.UUID            `json:"id"`
	RouteID      uuid.UUID            `json:"route_id"`
	DaysOfWeek   int16                `json:"days_of_week"`
	StartTime    string               `json:"start_time"`
	ValidFrom    coreModels.DateOnly  `json:"valid_from"`
	ValidUntil   *coreModels.DateOnly `json:"valid_until"`
	CalendarID   *uuid.UUID           `json:"calendar_id"`
	CalendarName *string              `json:"calendar_name"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

// Calendar is a named set of dates a schedule points at — a school year, the public holidays.
// OrganizationID is the school whose calendar it is, and NULL for one the operator keeps for the
// whole tenant.
type Calendar struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Name           string     `json:"name"`
	DatesCount     int32      `json:"dates_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CalendarDate is one date of one calendar: no_service suppresses the run, special_service adds one
// the weekday mask would not have produced.
type CalendarDate struct {
	Date  coreModels.DateOnly `json:"date"`
	Kind  string              `json:"kind"`
	Label *string             `json:"label"`
}

// RouteException is what happens differently over a range of dates (TRACK-018). Kind says what
// changes and Payload carries it in the shape that kind defines, which the SP checks on the way in —
// it stays raw JSON here because there is one shape per kind and no reader wants the other six.
type RouteException struct {
	ID        uuid.UUID           `json:"id"`
	RouteID   uuid.UUID           `json:"route_id"`
	DateFrom  coreModels.DateOnly `json:"date_from"`
	DateTo    coreModels.DateOnly `json:"date_to"`
	Kind      string              `json:"kind"`
	Payload   json.RawMessage     `json:"payload"`
	Reason    *string             `json:"reason"`
	CreatedBy *uuid.UUID          `json:"created_by"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

// RoutePreviewDay is one departure the plan produces: which schedule made it, at what time, running
// which stop list, with which bus and driver, and the exceptions that apply to the day. A date the
// route does not run — a weekday the mask excludes, or a calendar's no_service day — has no row.
type RoutePreviewDay struct {
	ServiceDate coreModels.DateOnly `json:"service_date"`
	ScheduleID  uuid.UUID           `json:"schedule_id"`
	StartTime   string              `json:"start_time"`
	VersionID   *uuid.UUID          `json:"version_id"`
	VehicleID   *uuid.UUID          `json:"vehicle_id"`
	DriverID    *uuid.UUID          `json:"driver_id"`
	Status      string              `json:"status"`
	Exceptions  json.RawMessage     `json:"exceptions"`
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

// Driver is a person who drives a carrier's vehicles (TRACK-006). CompanyName and UserEmail are
// joined on every read — list and single alike — so a row renders the carrier and the linked
// account without a second call. UserID points at an account whose tenant access level is `driver`
// (D1); it is nullable because a driver may exist long before anyone gives them one.
type Driver struct {
	ID               uuid.UUID            `json:"id"`
	TenantID         uuid.UUID            `json:"tenant_id"`
	CompanyID        uuid.UUID            `json:"company_id"`
	CompanyName      string               `json:"company_name"`
	UserID           *uuid.UUID           `json:"user_id"`
	UserEmail        *string              `json:"user_email"`
	FirstName        string               `json:"first_name"`
	LastName         string               `json:"last_name"`
	Phone            *string              `json:"phone"`
	LicenseNumber    string               `json:"license_number"`
	LicenseClass     *string              `json:"license_class"`
	LicenseExpiresOn *coreModels.DateOnly `json:"license_expires_on"`
	Status           string               `json:"status"` // active, inactive, suspended
	// ServiceBlocked rides on the list rows only: an expired document of a blocks_service type
	// stands against this driver (TRACK-016 D2).
	ServiceBlocked *bool     `json:"service_blocked,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Rider represents a student or employee using transport
type Rider struct {
	ID uuid.UUID `json:"id"`
	// The organization that sends the rider, not the carrier that drives them (TRACK-005 D1); the
	// carrier is reached through the route the rider is assigned to.
	OrganizationID uuid.UUID `json:"organization_id"`
	// OrganizationName rides on the list rows only (TRACK-003 D2), like Vehicle.CompanyName; the
	// single-rider SPs do not join the organization, so it is absent from create/update/get.
	OrganizationName *string `json:"organization_name,omitempty"`
	// OrganizationKind rides beside it (MOBILE-002 D3): school, company or other, as the operator
	// declared it — which is how the mobile app calls one list "students" and another "staff".
	OrganizationKind      *string    `json:"organization_kind,omitempty"`
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

// DocumentType is one line of a tenant's compliance policy (TRACK-016): what a vehicle or a driver
// must carry, how many days before expiry to warn, and whether an expired one stops them working.
type DocumentType struct {
	ID             uuid.UUID `json:"id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	AppliesTo      string    `json:"applies_to"` // vehicle, driver, both
	WarnDaysBefore int32     `json:"warn_days_before"`
	BlocksService  bool      `json:"blocks_service"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ComplianceDocument is one piece of paper filed against one vehicle or driver. SubjectType says
// which table SubjectID points at; SubjectName, TypeName and Status are resolved by the SP, so a row
// renders without a second call. Status is valid, expiring or expired, measured against the type's
// own WarnDaysBefore.
type ComplianceDocument struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	SubjectType string    `json:"subject_type"` // vehicle, driver
	SubjectID   uuid.UUID `json:"subject_id"`
	SubjectName *string   `json:"subject_name"`
	// DocumentTypeID and TypeName are the policy line this document answers.
	DocumentTypeID uuid.UUID            `json:"document_type_id"`
	TypeName       string               `json:"type_name"`
	Number         *string              `json:"number"`
	IssuedOn       *coreModels.DateOnly `json:"issued_on"`
	ExpiresOn      *coreModels.DateOnly `json:"expires_on"`
	// The three file columns are what the browser is told on download; the stored file is named
	// after the document's id (TRACK-016 D3).
	FileName  *string    `json:"file_name"`
	FileSize  *int64     `json:"file_size"`
	FileExt   *string    `json:"file_ext"`
	Notes     *string    `json:"notes"`
	Status    string     `json:"status"`
	WarnedAt  *time.Time `json:"warned_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// DocumentAlert is one line of the expiry digest (TRACK-016 D1): a document the job has just claimed
// by stamping warned_at, described well enough for the email without a second read.
type DocumentAlert struct {
	ID          uuid.UUID `json:"id"`
	SubjectType string    `json:"subject_type"`
	SubjectName *string   `json:"subject_name"`
	TypeName    string    `json:"type_name"`
	Number      *string   `json:"number"`
	ExpiresOn   time.Time `json:"expires_on"`
	// DaysLeft is negative once the document has expired — the digest says "act on these", not
	// "these are about to happen".
	DaysLeft int32 `json:"days_left"`
}
