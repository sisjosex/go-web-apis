package repositories

import (
	"context"
	"strings"
	"time"

	coreServices "josex/web/modules/core/services"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

type TrackingRepository struct {
	dbService coreServices.DatabaseService
}

func NewTrackingRepository(dbService coreServices.DatabaseService) *TrackingRepository {
	return &TrackingRepository{
		dbService: dbService,
	}
}

// UpdateVehicleLocation inserts GPS coordinates for a vehicle
func (r *TrackingRepository) UpdateVehicleLocation(ctx context.Context, dto *models.UpdateLocationDto) (*models.CurrentLocationResponse, error) {
	var recordedAt time.Time
	if dto.RecordedAt != nil {
		parsedTime, err := time.Parse(time.RFC3339, *dto.RecordedAt)
		if err == nil {
			recordedAt = parsedTime
		} else {
			recordedAt = time.Now()
		}
	} else {
		recordedAt = time.Now()
	}

	var locationID uuid.UUID
	var vehicleID uuid.UUID
	var latitude, longitude float64
	var returnedRecordedAt time.Time

	var speed, heading, altitude, accuracy *float64

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_record_vehicle_location($1, $2, $3, $4, $5, $6, $7, $8)`,
		dto.VehicleID,
		dto.Latitude,
		dto.Longitude,
		dto.Speed,
		dto.Heading,
		dto.Altitude,
		dto.Accuracy,
		recordedAt,
	).Scan(&locationID, &vehicleID, &latitude, &longitude, &speed, &heading, &altitude, &accuracy, &returnedRecordedAt)

	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.LocationUpdateFailed, Err: err}
	}

	// Get vehicle details
	var licensePlate string
	err = r.dbService.QueryRow(ctx,
		`SELECT plate_number FROM tracking.sp_get_vehicle_plate($1)`,
		vehicleID,
	).Scan(&licensePlate)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: err}
	}

	recordedAtStr := returnedRecordedAt.Format(time.RFC3339)
	ageSeconds := int32(time.Since(returnedRecordedAt).Seconds())

	return &models.CurrentLocationResponse{
		VehicleID:    vehicleID,
		LicensePlate: licensePlate,
		Latitude:     &latitude,
		Longitude:    &longitude,
		Speed:        dto.Speed,
		Heading:      dto.Heading,
		RecordedAt:   &recordedAtStr,
		AgeSeconds:   &ageSeconds,
	}, nil
}

// GetVehicleCurrentLocation retrieves the most recent GPS location
func (r *TrackingRepository) GetVehicleCurrentLocation(ctx context.Context, vehicleID uuid.UUID) (*models.CurrentLocationResponse, error) {
	var retVehicleID uuid.UUID
	var licensePlate string
	var latitude, longitude *float64
	var speed, heading *float64
	var recordedAt *time.Time
	var ageSeconds *int32

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_get_vehicle_current_location($1)`,
		vehicleID,
	).Scan(&retVehicleID, &licensePlate, &latitude, &longitude, &speed, &heading, &recordedAt, &ageSeconds)

	if err != nil {
		return nil, err
	}

	var recordedAtStr *string
	if recordedAt != nil {
		str := recordedAt.Format(time.RFC3339)
		recordedAtStr = &str
	}

	return &models.CurrentLocationResponse{
		VehicleID:    retVehicleID,
		LicensePlate: licensePlate,
		Latitude:     latitude,
		Longitude:    longitude,
		Speed:        speed,
		Heading:      heading,
		RecordedAt:   recordedAtStr,
		AgeSeconds:   ageSeconds,
	}, nil
}

// RecordRideEvent records a boarding/arrival event
func (r *TrackingRepository) RecordRideEvent(ctx context.Context, dto *models.RecordEventDto, createdBy *uuid.UUID) (*models.EventRecordedResponse, error) {
	var eventID uuid.UUID
	var riderID uuid.UUID
	var vehicleID uuid.UUID
	var routeID uuid.UUID
	var eventType string
	var eventTime time.Time
	var locationLatitude *float64
	var locationLongitude *float64
	var notes *string
	var returnedCreatedBy *uuid.UUID
	var createdAt time.Time

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_record_ride_event($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR(50), $5::DECIMAL, $6::DECIMAL, $7::TEXT, $8::UUID)`,
		dto.RiderID,
		dto.VehicleID,
		dto.RouteID,
		dto.EventType,
		dto.Latitude,
		dto.Longitude,
		dto.Notes,
		createdBy,
	).Scan(&eventID, &riderID, &vehicleID, &routeID, &eventType, &eventTime, &locationLatitude, &locationLongitude, &notes, &returnedCreatedBy, &createdAt)

	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "TR0001":
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound}
		case "TR0002":
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderNotFound}
		case "TR0003":
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound}
		default:
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.EventRecordFailed, Err: err}
		}
	}

	// TODO: Get rider name from riders table
	riderName := ""

	return &models.EventRecordedResponse{
		EventID:   eventID,
		RiderID:   riderID,
		RiderName: riderName,
		EventType: eventType,
		EventTime: eventTime.Format(time.RFC3339),
		StopName:  nil,
	}, nil
}

// GetRouteRealtimeStatus gets comprehensive route status
func (r *TrackingRepository) GetRouteRealtimeStatus(ctx context.Context, routeID uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	var retRouteID uuid.UUID
	var routeName string
	var vehicleID *uuid.UUID
	var licensePlate *string
	var currentLatitude, currentLongitude *float64
	var currentSpeed *float64
	var locationAgeSeconds *int32
	var totalRiders, boardedCount, arrivedCount, noShowCount, pendingCount, activeAlerts int32

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_get_route_realtime_status($1)`,
		routeID,
	).Scan(
		&retRouteID,
		&routeName,
		&vehicleID,
		&licensePlate,
		&currentLatitude,
		&currentLongitude,
		&currentSpeed,
		&locationAgeSeconds,
		&totalRiders,
		&boardedCount,
		&arrivedCount,
		&noShowCount,
		&pendingCount,
		&activeAlerts,
	)

	if err != nil {
		return nil, err
	}

	return &models.RouteRealtimeStatusResponse{
		RouteID:            retRouteID,
		RouteName:          routeName,
		VehicleID:          vehicleID,
		LicensePlate:       licensePlate,
		CurrentLatitude:    currentLatitude,
		CurrentLongitude:   currentLongitude,
		CurrentSpeed:       currentSpeed,
		LocationAgeSeconds: locationAgeSeconds,
		TotalRiders:        totalRiders,
		BoardedCount:       boardedCount,
		ArrivedCount:       arrivedCount,
		NoShowCount:        noShowCount,
		PendingCount:       pendingCount,
		ActiveAlerts:       activeAlerts,
	}, nil
}

// GetRiderStatus gets rider status for guardian view
func (r *TrackingRepository) GetRiderStatus(ctx context.Context, riderID uuid.UUID) (*models.RiderStatusResponse, error) {
	var retRiderID uuid.UUID
	var riderName string
	var routeID *uuid.UUID
	var routeName *string
	var vehicleID *uuid.UUID
	var licensePlate *string
	var driverName *string
	var vehicleLatitude, vehicleLongitude *float64
	var vehicleSpeed *float64
	var locationAgeSeconds *int32
	var lastEventType *string
	var lastEventTime *time.Time
	var lastEventNotes *string
	var lastEventStop *string
	var scheduledPickupStop *string
	var scheduledDropoffStop *string
	var activeAlerts int32

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_get_rider_status($1)`,
		riderID,
	).Scan(
		&retRiderID,
		&riderName,
		&routeID,
		&routeName,
		&vehicleID,
		&licensePlate,
		&driverName,
		&vehicleLatitude,
		&vehicleLongitude,
		&vehicleSpeed,
		&locationAgeSeconds,
		&lastEventType,
		&lastEventTime,
		&lastEventNotes,
		&lastEventStop,
		&scheduledPickupStop,
		&scheduledDropoffStop,
		&activeAlerts,
	)

	if err != nil {
		return nil, err
	}

	var lastEventTimeStr *string
	if lastEventTime != nil {
		str := lastEventTime.Format(time.RFC3339)
		lastEventTimeStr = &str
	}

	return &models.RiderStatusResponse{
		RiderID:              retRiderID,
		RiderName:            riderName,
		RouteID:              routeID,
		RouteName:            routeName,
		VehicleID:            vehicleID,
		LicensePlate:         licensePlate,
		DriverName:           driverName,
		VehicleLatitude:      vehicleLatitude,
		VehicleLongitude:     vehicleLongitude,
		VehicleSpeed:         vehicleSpeed,
		LocationAgeSeconds:   locationAgeSeconds,
		LastEventType:        lastEventType,
		LastEventTime:        lastEventTimeStr,
		LastEventNotes:       lastEventNotes,
		LastEventStop:        lastEventStop,
		ScheduledPickupStop:  scheduledPickupStop,
		ScheduledDropoffStop: scheduledDropoffStop,
		ActiveAlerts:         activeAlerts,
	}, nil
}

// CreateRouteAlert creates a route delay/breakdown alert
func (r *TrackingRepository) CreateRouteAlert(ctx context.Context, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error) {
	severity := "medium"
	if dto.Severity != "" {
		severity = dto.Severity
	}

	var alertID uuid.UUID
	var routeID uuid.UUID
	var vehicleID *uuid.UUID
	var alertType string
	var title string
	var message string
	var retSeverity string
	var estimatedDelayMinutes *int32
	var status string
	var retCreatedBy uuid.UUID
	var createdAt time.Time

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_create_route_alert($1::UUID, $2::UUID, $3::VARCHAR(50), $4::VARCHAR(255), $5::TEXT, $6::VARCHAR(50), $7::INTEGER, $8::UUID)`,
		dto.RouteID,
		dto.VehicleID,
		dto.AlertType,
		dto.Title,
		dto.Message,
		severity,
		dto.EstimatedDelayMinutes,
		createdBy,
	).Scan(&alertID, &routeID, &vehicleID, &alertType, &title, &message, &retSeverity, &estimatedDelayMinutes, &status, &retCreatedBy, &createdAt)

	if err != nil {
		if err.Error() == "TR0003" {
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound}
		}
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AlertCreateFailed, Err: err}
	}

	return &models.AlertCreatedResponse{
		AlertID:               alertID,
		RouteID:               routeID,
		VehicleID:             vehicleID,
		AlertType:             alertType,
		Title:                 title,
		Message:               message,
		Severity:              retSeverity,
		EstimatedDelayMinutes: estimatedDelayMinutes,
		Status:                status,
		CreatedBy:             retCreatedBy,
		CreatedAt:             createdAt.Format(time.RFC3339),
	}, nil
}

// ==================== COMPANIES CRUD ====================

func (r *TrackingRepository) CreateCompany(ctx context.Context, dto *models.CreateCompanyDto) (*models.TransportCompany, error) {
	var company models.TransportCompany

	// Auto-generate registration number if not provided
	registrationNumber := strings.TrimSpace(dto.RegistrationNumber)
	if registrationNumber == "" {
		registrationNumber = "REG-" + uuid.New().String()[:12]
	}

	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_create_company($1, $2, $3, $4, $5, $6, $7, $8)`,
		dto.Name, dto.Email, dto.Phone, dto.Address, dto.City, dto.Country, registrationNumber, dto.Status,
	).Scan(&company.ID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyCreateFailed, Err: err}
	}
	return &company, nil
}

func (r *TrackingRepository) UpdateCompany(ctx context.Context, companyID uuid.UUID, dto *models.UpdateCompanyDto) (*models.TransportCompany, error) {
	var company models.TransportCompany
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_update_company($1, $2, $3, $4, $5, $6, $7)`,
		companyID, dto.Name, dto.Email, dto.Phone, dto.Address, dto.City, dto.Country,
	).Scan(&company.ID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyUpdateFailed, Err: err}
	}
	return &company, nil
}

func (r *TrackingRepository) ListCompanies(ctx context.Context, registrationNumber *string, status *string) ([]*models.TransportCompany, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_companies($1::VARCHAR(255), $2::VARCHAR(50))`, registrationNumber, status)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyListFailed, Err: err}
	}
	defer rows.Close()
	var companies []*models.TransportCompany
	for rows.Next() {
		var c models.TransportCompany
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Address, &c.City, &c.Country, &c.RegistrationNumber, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		companies = append(companies, &c)
	}
	return companies, nil
}

func (r *TrackingRepository) GetCompany(ctx context.Context, companyID uuid.UUID) (*models.TransportCompany, error) {
	var company models.TransportCompany
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_company($1)`, companyID).Scan(&company.ID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyNotFound, Err: err}
	}
	return &company, nil
}

func (r *TrackingRepository) DeleteCompany(ctx context.Context, companyID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_company($1)`, companyID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.CompanyDeleteFailed, Err: err}
	}
	return nil
}

// ==================== VEHICLES CRUD ====================

func (r *TrackingRepository) CreateVehicle(ctx context.Context, dto *models.CreateVehicleDto) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_vehicle($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		dto.CompanyID, dto.PlateNumber, dto.VehicleType, dto.Brand, dto.Model, dto.Year, dto.Capacity, dto.GPSDeviceID, dto.Status,
	).Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleCreateFailed, Err: err}
	}
	return &v, nil
}

func (r *TrackingRepository) UpdateVehicle(ctx context.Context, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_vehicle($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		vehicleID, dto.PlateNumber, nil, dto.Brand, dto.Model, dto.Year, dto.Capacity, dto.GPSDeviceID, dto.Status,
	).Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleUpdateFailed, Err: err}
	}
	return &v, nil
}

func (r *TrackingRepository) ListVehicles(ctx context.Context, companyID *uuid.UUID, vehicleType *string, status *string) ([]*models.Vehicle, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_vehicles($1::UUID, $2::VARCHAR(50), $3::VARCHAR(50))`, companyID, vehicleType, status)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleListFailed, Err: err}
	}
	defer rows.Close()
	var vehicles []*models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, &v)
	}
	return vehicles, nil
}

func (r *TrackingRepository) GetVehicle(ctx context.Context, vehicleID uuid.UUID) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_vehicle($1)`, vehicleID).Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: err}
	}
	return &v, nil
}

func (r *TrackingRepository) DeleteVehicle(ctx context.Context, vehicleID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_vehicle($1)`, vehicleID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.VehicleDeleteFailed, Err: err}
	}
	return nil
}

// ==================== ROUTES CRUD ====================

func (r *TrackingRepository) CreateRoute(ctx context.Context, dto *models.CreateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_route($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		dto.CompanyID, dto.RouteName, dto.OriginAddress, dto.DestinationAddress, dto.RouteCode, dto.VehicleID,
		dto.OriginLat, dto.OriginLng, dto.DestinationLat, dto.DestinationLng, dto.ScheduleType,
		dto.ScheduledStartTime, dto.ScheduledEndTime, dto.EstimatedDurationMinutes,
	).Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteCreateFailed, Err: err}
	}
	return &rt, nil
}

func (r *TrackingRepository) UpdateRoute(ctx context.Context, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_route($1, $2, $3, $4)`,
		routeID, dto.RouteName, dto.VehicleID, dto.IsActive,
	).Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteUpdateFailed, Err: err}
	}
	return &rt, nil
}

func (r *TrackingRepository) ListRoutes(ctx context.Context, companyID *uuid.UUID, isActive *bool) ([]*models.Route, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_routes($1::UUID, $2::BOOLEAN)`, companyID, isActive)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteListFailed, Err: err}
	}
	defer rows.Close()
	var routes []*models.Route
	for rows.Next() {
		var rt models.Route
		if err := rows.Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt); err != nil {
			return nil, err
		}
		routes = append(routes, &rt)
	}
	return routes, nil
}

func (r *TrackingRepository) GetRoute(ctx context.Context, routeID uuid.UUID) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_route($1)`, routeID).Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: err}
	}
	return &rt, nil
}

func (r *TrackingRepository) DeleteRoute(ctx context.Context, routeID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route($1)`, routeID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.RouteDeleteFailed, Err: err}
	}
	return nil
}

// ==================== ROUTE STOPS CRUD ====================

func (r *TrackingRepository) CreateRouteStop(ctx context.Context, dto *models.CreateRouteStopDto) (*models.RouteStop, error) {
	var stop models.RouteStop
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_route_stop($1, $2, $3, $4, $5, $6, $7)`,
		dto.RouteID, dto.StopName, dto.StopAddress, dto.Latitude, dto.Longitude, dto.SequenceOrder, dto.ScheduledTime,
	).Scan(&stop.ID, &stop.RouteID, &stop.StopName, &stop.Address, &stop.Latitude, &stop.Longitude, &stop.StopOrder, &stop.ScheduledArrivalOffsetMinutes, &stop.CreatedAt, &stop.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteStopCreateFailed, Err: err}
	}
	return &stop, nil
}

func (r *TrackingRepository) ListRouteStops(ctx context.Context, routeID uuid.UUID) ([]*models.RouteStop, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_route_stops($1)`, routeID)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteStopListFailed, Err: err}
	}
	defer rows.Close()
	var stops []*models.RouteStop
	for rows.Next() {
		var stop models.RouteStop
		if err := rows.Scan(&stop.ID, &stop.RouteID, &stop.StopName, &stop.Address, &stop.Latitude, &stop.Longitude, &stop.StopOrder, &stop.ScheduledArrivalOffsetMinutes, &stop.CreatedAt, &stop.UpdatedAt); err != nil {
			return nil, err
		}
		stops = append(stops, &stop)
	}
	return stops, nil
}

func (r *TrackingRepository) DeleteRouteStop(ctx context.Context, stopID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route_stop($1)`, stopID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.RouteStopDeleteFailed, Err: err}
	}
	return nil
}

// ==================== RIDERS CRUD ====================

func (r *TrackingRepository) CreateRider(ctx context.Context, dto *models.CreateRiderDto) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_rider($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		dto.CompanyID, dto.RiderType, dto.FirstName, dto.LastName, dto.IdentificationNumber,
		dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		dto.GuardianUserID, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail, dto.Address,
	).Scan(&rider.ID, &rider.CompanyID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderCreateFailed, Err: err}
	}
	return &rider, nil
}

func (r *TrackingRepository) UpdateRider(ctx context.Context, riderID uuid.UUID, dto *models.UpdateRiderDto) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_rider($1, $2::VARCHAR(50), $3::VARCHAR(255), $4::VARCHAR(255), $5::VARCHAR(50), $6::UUID, $7::VARCHAR(255), $8::VARCHAR(50), $9::VARCHAR(255), $10::VARCHAR(500), $11::BOOLEAN)`,
		riderID, dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		dto.GuardianUserID, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail,
		dto.Address, dto.IsActive,
	).Scan(&rider.ID, &rider.CompanyID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderUpdateFailed, Err: err}
	}
	return &rider, nil
}

func (r *TrackingRepository) ListRiders(ctx context.Context, companyID *uuid.UUID, guardianUserID *uuid.UUID, riderType *string, isActive *bool) ([]*models.Rider, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_riders($1::UUID, $2::UUID, $3::VARCHAR(50), $4::BOOLEAN)`, companyID, guardianUserID, riderType, isActive)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderListFailed, Err: err}
	}
	defer rows.Close()
	var riders []*models.Rider
	for rows.Next() {
		var rider models.Rider
		if err := rows.Scan(&rider.ID, &rider.CompanyID, &rider.RiderType, &rider.FirstName, &rider.LastName,
			&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
			&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
			&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt); err != nil {
			return nil, err
		}
		riders = append(riders, &rider)
	}
	return riders, nil
}

func (r *TrackingRepository) GetRider(ctx context.Context, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_rider($1::UUID, $2::UUID)`, riderID, guardianUserID).Scan(
		&rider.ID, &rider.CompanyID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderNotFound, Err: err}
	}
	return &rider, nil
}

func (r *TrackingRepository) DeleteRider(ctx context.Context, riderID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_rider($1)`, riderID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.RiderDeleteFailed, Err: err}
	}
	return nil
}

// ==================== ASSIGNMENTS CRUD ====================

func (r *TrackingRepository) AssignRider(ctx context.Context, dto *models.AssignRiderDto) (*models.RiderAssignment, error) {
	var assignment models.RiderAssignment
	var status string
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_assign_rider($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::VARCHAR(50))`,
		dto.RiderID, dto.RouteID, dto.PickupStopID, dto.DropoffStopID, "active",
	).Scan(&assignment.ID, &assignment.RiderID, &assignment.RouteID, &assignment.PickupStopID, &assignment.DropoffStopID, &status, &assignment.CreatedAt, &assignment.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentCreateFailed, Err: err}
	}
	assignment.IsActive = (status == "active")
	assignment.AssignedAt = assignment.CreatedAt
	return &assignment, nil
}

func (r *TrackingRepository) UnassignRider(ctx context.Context, assignmentID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_unassign_rider($1)`, assignmentID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.AssignmentDeleteFailed, Err: err}
	}
	return nil
}

func (r *TrackingRepository) ListRiderAssignments(ctx context.Context, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_rider_assignments($1::UUID, $2::UUID, $3::BOOLEAN)`, riderID, routeID, isActive)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentListFailed, Err: err}
	}
	defer rows.Close()
	var assignments []*models.RiderAssignment
	for rows.Next() {
		var a models.RiderAssignment
		var status string
		if err := rows.Scan(&a.ID, &a.RiderID, &a.RouteID, &a.PickupStopID, &a.DropoffStopID, &status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.IsActive = (status == "active")
		a.AssignedAt = a.CreatedAt
		assignments = append(assignments, &a)
	}
	return assignments, nil
}

// === Client Access Management ===

// GrantClientAccess grants read/write access to a client tenant for a company
func (r *TrackingRepository) GrantClientAccess(ctx context.Context, companyID uuid.UUID, dto *models.GrantClientAccessDto, grantedBy uuid.UUID, userTenantID uuid.UUID) (*models.CompanyClientAccess, error) {
	accessLevel := "read_only"
	if dto.AccessLevel != "" {
		accessLevel = dto.AccessLevel
	}

	var access models.CompanyClientAccess
	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_grant_client_access($1, $2, $3, $4, $5, $6)`,
		companyID,
		dto.ClientTenantID,
		accessLevel,
		grantedBy,
		dto.Notes,
		userTenantID,
	).Scan(
		&access.ID,
		&access.CompanyID,
		&access.ClientTenantID,
		&access.AccessLevel,
		&access.GrantedAt,
		&access.GrantedBy,
		&access.IsActive,
		&access.Notes,
	)

	if err != nil {
		return nil, err
	}

	return &access, nil
}

// RevokeClientAccess revokes access for a client tenant
func (r *TrackingRepository) RevokeClientAccess(ctx context.Context, companyID uuid.UUID, clientTenantID uuid.UUID, revokedBy uuid.UUID, userTenantID uuid.UUID) error {
	_, err := r.dbService.Execute(
		ctx,
		`SELECT tracking.sp_revoke_client_access($1, $2, $3, $4)`,
		companyID,
		clientTenantID,
		revokedBy,
		userTenantID,
	)

	if err != nil {
		return err
	}

	return nil
}

// ListCompanyClients lists all clients with access to a company
func (r *TrackingRepository) ListCompanyClients(ctx context.Context, companyID uuid.UUID, userTenantID uuid.UUID) ([]*models.CompanyClientAccess, error) {
	rows, err := r.dbService.Query(
		ctx,
		`SELECT * FROM tracking.sp_list_company_clients($1, $2)`,
		companyID,
		userTenantID,
	)

	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.ClientAccessListFailed, Err: err}
	}
	defer rows.Close()

	var clients []*models.CompanyClientAccess = make([]*models.CompanyClientAccess, 0)
	for rows.Next() {
		var access models.CompanyClientAccess
		if err := rows.Scan(
			&access.ID,
			&access.CompanyID,
			&access.ClientTenantID,
			&access.ClientName,
			&access.AccessLevel,
			&access.GrantedAt,
			&access.GrantedBy,
			&access.RevokedAt,
			&access.RevokedBy,
			&access.IsActive,
			&access.Notes,
		); err != nil {
			return nil, err
		}
		clients = append(clients, &access)
	}

	return clients, nil
}
