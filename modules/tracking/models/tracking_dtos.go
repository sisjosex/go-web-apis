package models

import "github.com/google/uuid"

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
	EventType string     `json:"event_type" binding:"required,oneof=check_in checkout no_show emergency"`
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
	RouteID            uuid.UUID  `json:"route_id"`
	RouteName          string     `json:"route_name"`
	VehicleID          *uuid.UUID `json:"vehicle_id"`
	LicensePlate       *string    `json:"license_plate"`
	CurrentLatitude    *float64   `json:"current_latitude"`
	CurrentLongitude   *float64   `json:"current_longitude"`
	CurrentSpeed       *float64   `json:"current_speed"`
	LocationAgeSeconds *int32     `json:"location_age_seconds"`
	TotalRiders        int32      `json:"total_riders"`
	BoardedCount       int32      `json:"boarded_count"`
	ArrivedCount       int32      `json:"arrived_count"`
	NoShowCount        int32      `json:"no_show_count"`
	PendingCount       int32      `json:"pending_count"`
	ActiveAlerts       int32      `json:"active_alerts"`
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
	AlertType             string     `json:"alert_type" binding:"required,oneof=delay breakdown traffic cancellation emergency other"`
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

// === DTOs for Vehicle Management ===

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

// CreateRiderDto represents request to create rider
type CreateRiderDto struct {
	CompanyID             uuid.UUID  `json:"company_id" binding:"required,uuidv4"`
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

// === DTOs for Route Stop Management ===

// CreateRouteStopDto represents request to create route stop
type CreateRouteStopDto struct {
	RouteID       uuid.UUID `json:"route_id" binding:"required,uuidv4"`
	StopName      string    `json:"stop_name" binding:"required,min=3,max=255" conform:"trim"`
	StopAddress   string    `json:"stop_address" binding:"required,min=5,max=500" conform:"trim"`
	Latitude      float64   `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude     float64   `json:"longitude" binding:"required,min=-180,max=180"`
	SequenceOrder int32     `json:"sequence_order" binding:"required,min=1"`
	ScheduledTime *string   `json:"scheduled_time" binding:"omitempty"` // HH:MM:SS
}

// === DTOs for Rider Assignment ===

// AssignRiderDto represents request to assign rider to route
type AssignRiderDto struct {
	RiderID       uuid.UUID  `json:"rider_id" binding:"required,uuidv4"`
	RouteID       uuid.UUID  `json:"route_id" binding:"required,uuidv4"`
	PickupStopID  *uuid.UUID `json:"pickup_stop_id" binding:"omitempty,uuidv4"`
	DropoffStopID *uuid.UUID `json:"dropoff_stop_id" binding:"omitempty,uuidv4"`
}

// === DTOs for Client Access Management ===

// GrantClientAccessDto represents request to grant access to a client
type GrantClientAccessDto struct {
	ClientTenantID uuid.UUID `json:"client_tenant_id" binding:"required,uuidv4" conform:"trim"`
	AccessLevel    string    `json:"access_level" binding:"omitempty,oneof=read_only read_write" conform:"trim,lowercase"`
	Notes          *string   `json:"notes" binding:"omitempty,max=500" conform:"trim"`
}
