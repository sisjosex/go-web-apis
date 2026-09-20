package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	coreServices "josex/web/modules/core/services"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type TrackingRepository struct {
	dbService coreServices.DatabaseService
}

func NewTrackingRepository(dbService coreServices.DatabaseService) *TrackingRepository {
	return &TrackingRepository{
		dbService: dbService,
	}
}

// scopedErr classifies an error from a scope-aware SP (TRACK-015 D1). The scope refusal and the
// not-found raises are the only exceptions those SPs make; anything else keeps the endpoint's own
// failure code. A row out of scope arrives as not-found, so the caller cannot tell a foreign
// organization's rider from one that does not exist.
func scopedErr(err error, fallback string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case trackingErrors.OrganizationScopeDenied:
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationScopeDenied, Err: pgErr}
		case "rider.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RiderNotFound, Err: pgErr}
		case "route.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
		case "organization.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallback, Err: err}
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
		`SELECT * FROM tracking.sp_record_ride_event($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR(50), $5::DECIMAL, $6::DECIMAL, $7::TEXT, $8::UUID, $9::UUID)`,
		dto.RiderID,
		dto.VehicleID,
		dto.RouteID,
		dto.EventType,
		dto.Latitude,
		dto.Longitude,
		dto.Notes,
		createdBy,
		dto.StopID,
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
func (r *TrackingRepository) GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	var retRouteID uuid.UUID
	var routeName string
	var vehicleID *uuid.UUID
	var licensePlate *string
	var driverName *string
	var currentLatitude, currentLongitude *float64
	var currentSpeed *float64
	var locationAgeSeconds *int32
	var totalRiders, boardedCount, arrivedCount, noShowCount, pendingCount, activeAlerts int32

	err := r.dbService.QueryRow(
		ctx,
		`SELECT * FROM tracking.sp_get_route_realtime_status($1, $2, $3)`,
		tenantID,
		routeID,
		scopeUserID,
	).Scan(
		&retRouteID,
		&routeName,
		&vehicleID,
		&licensePlate,
		&driverName,
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
		return nil, scopedErr(err, trackingErrors.RouteNotFound)
	}

	return &models.RouteRealtimeStatusResponse{
		RouteID:            retRouteID,
		RouteName:          routeName,
		VehicleID:          vehicleID,
		LicensePlate:       licensePlate,
		DriverName:         driverName,
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
func (r *TrackingRepository) GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) (*models.RiderStatusResponse, error) {
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
		`SELECT * FROM tracking.sp_get_rider_status($1, $2, $3, $4)`,
		tenantID,
		riderID,
		scopeUserID,
		guardianUserID,
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
		return nil, scopedErr(err, trackingErrors.RiderNotFound)
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
func (r *TrackingRepository) CreateRouteAlert(ctx context.Context, tenantID uuid.UUID, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error) {
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
		`SELECT * FROM tracking.sp_create_route_alert($1::UUID, $2::UUID, $3::UUID, $4::VARCHAR(50), $5::VARCHAR(255), $6::TEXT, $7::VARCHAR(50), $8::INTEGER, $9::UUID)`,
		tenantID,
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

func (r *TrackingRepository) CreateCompany(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCompanyDto) (*models.TransportCompany, error) {
	var company models.TransportCompany

	registrationNumber := strings.TrimSpace(dto.RegistrationNumber)
	if registrationNumber == "" {
		registrationNumber = "REG-" + uuid.New().String()[:12]
	}

	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_create_company($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID, dto.Name, dto.Email, dto.Phone, dto.Address, dto.City, dto.Country, registrationNumber, dto.Status,
	).Scan(&company.ID, &company.TenantID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyCreateFailed, Err: err}
	}
	return &company, nil
}

func (r *TrackingRepository) UpdateCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, dto *models.UpdateCompanyDto) (*models.TransportCompany, error) {
	var company models.TransportCompany
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_update_company($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, companyID, dto.Name, dto.Email, dto.Phone, dto.Address, dto.City, dto.Country,
	).Scan(&company.ID, &company.TenantID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyUpdateFailed, Err: err}
	}
	return &company, nil
}

// ListCompanies returns one page of the tenant's companies plus the total the same filters
// match. total_count comes back on every row and stays 0 when the page is empty.
func (r *TrackingRepository) ListCompanies(ctx context.Context, tenantID uuid.UUID, query models.ListCompaniesQuery) ([]*models.TransportCompany, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_companies($1::UUID, $2::VARCHAR, $3::VARCHAR, $4::INT, $5::INT)`,
		tenantID, query.Search, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.CompanyListFailed, Err: err}
	}
	defer rows.Close()
	companies := []*models.TransportCompany{}
	var totalCount int64
	for rows.Next() {
		var c models.TransportCompany
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.Email, &c.Phone, &c.Address, &c.City, &c.Country, &c.RegistrationNumber, &c.Status, &c.CreatedAt, &c.UpdatedAt, &totalCount); err != nil {
			return nil, 0, err
		}
		companies = append(companies, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return companies, totalCount, nil
}

func (r *TrackingRepository) GetCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) (*models.TransportCompany, error) {
	var company models.TransportCompany
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_company($1, $2)`, tenantID, companyID).Scan(
		&company.ID, &company.TenantID, &company.Name, &company.Email, &company.Phone, &company.Address, &company.City, &company.Country, &company.RegistrationNumber, &company.Status, &company.CreatedAt, &company.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CompanyNotFound, Err: err}
	}
	return &company, nil
}

func (r *TrackingRepository) DeleteCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_company($1, $2)`, tenantID, companyID).Scan(&deleted)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "company.not-found":
				return &trackingErrors.TrackingError{Code: trackingErrors.CompanyNotFound, Err: pgErr}
			case "company.has-vehicles":
				return &trackingErrors.TrackingError{Code: trackingErrors.CompanyHasVehicles, Err: pgErr}
			}
		}
		return &trackingErrors.TrackingError{Code: trackingErrors.CompanyDeleteFailed, Err: err}
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.CompanyDeleteFailed}
	}
	return nil
}

// ==================== VEHICLES CRUD ====================

func (r *TrackingRepository) CreateVehicle(ctx context.Context, tenantID uuid.UUID, dto *models.CreateVehicleDto) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_vehicle($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenantID, dto.CompanyID, dto.PlateNumber, dto.VehicleType, dto.Brand, dto.Model, dto.Year, dto.Capacity, dto.GPSDeviceID, dto.Status,
	).Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleCreateFailed, Err: err}
	}
	return &v, nil
}

func (r *TrackingRepository) UpdateVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_vehicle($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenantID, vehicleID, dto.PlateNumber, nil, dto.Brand, dto.Model, dto.Year, dto.Capacity, dto.GPSDeviceID, dto.Status,
	).Scan(&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleUpdateFailed, Err: err}
	}
	return &v, nil
}

// ListVehicles returns one page of the tenant's vehicles plus the total the same filters match.
// query.CompanyID nil lists the whole fleet (TRACK-001 D2); every row carries its company name.
func (r *TrackingRepository) ListVehicles(ctx context.Context, tenantID uuid.UUID, query models.ListVehiclesQuery) ([]*models.Vehicle, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_vehicles($1::UUID, $2::UUID, $3::VARCHAR, $4::VARCHAR, $5::VARCHAR, $6::INT, $7::INT)`,
		tenantID, query.CompanyID, query.VehicleType, query.Status, query.Search, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.VehicleListFailed, Err: err}
	}
	defer rows.Close()
	vehicles := []*models.Vehicle{}
	var totalCount int64
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.CompanyID, &v.CompanyName, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.ServiceBlocked, &v.CreatedAt, &v.UpdatedAt, &totalCount); err != nil {
			return nil, 0, err
		}
		vehicles = append(vehicles, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return vehicles, totalCount, nil
}

func (r *TrackingRepository) GetVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) (*models.Vehicle, error) {
	var v models.Vehicle
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_vehicle($1, $2)`, tenantID, vehicleID).Scan(
		&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: err}
	}
	return &v, nil
}

func (r *TrackingRepository) DeleteVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_vehicle($1, $2)`, tenantID, vehicleID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.VehicleDeleteFailed, Err: err}
	}
	return nil
}

// ==================== ROUTES CRUD ====================

func (r *TrackingRepository) CreateRoute(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_route($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		tenantID, dto.CompanyID, dto.RouteName, dto.OriginAddress, dto.DestinationAddress, dto.RouteCode, dto.VehicleID,
		dto.OriginLat, dto.OriginLng, dto.DestinationLat, dto.DestinationLng, dto.ScheduleType,
		dto.ScheduledStartTime, dto.ScheduledEndTime, dto.EstimatedDurationMinutes,
	).Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteCreateFailed, Err: err}
	}
	return &rt, nil
}

func (r *TrackingRepository) UpdateRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_route($1, $2, $3, $4, $5)`,
		tenantID, routeID, dto.RouteName, dto.VehicleID, dto.IsActive,
	).Scan(&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteUpdateFailed, Err: err}
	}
	return &rt, nil
}

func (r *TrackingRepository) ListRoutes(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, isActive *bool, scopeUserID *uuid.UUID) ([]*models.Route, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_routes($1::UUID, $2::UUID, $3::BOOLEAN, $4::UUID)`, tenantID, companyID, isActive, scopeUserID)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RouteListFailed)
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
	// pgx surfaces an SP exception here, not on Query: without this the refusal would read as an
	// empty list and answer 200.
	if err := rows.Err(); err != nil {
		return nil, scopedErr(err, trackingErrors.RouteListFailed)
	}
	return routes, nil
}

func (r *TrackingRepository) GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_route($1, $2, $3)`, tenantID, routeID, scopeUserID).Scan(
		&rt.ID, &rt.CompanyID, &rt.VehicleID, &rt.RouteName, &rt.RouteCode, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng, &rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.ScheduleType, &rt.ScheduledStartTime, &rt.ScheduledEndTime, &rt.EstimatedDurationMinutes, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RouteNotFound)
	}
	return &rt, nil
}

func (r *TrackingRepository) DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route($1, $2)`, tenantID, routeID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.RouteDeleteFailed, Err: err}
	}
	return nil
}

// ==================== ROUTE STOPS CRUD ====================

func (r *TrackingRepository) CreateRouteStop(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteStopDto) (*models.RouteStop, error) {
	var stop models.RouteStop
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_route_stop($1, $2, $3, $4, $5, $6, $7, $8)`,
		tenantID, dto.RouteID, dto.StopName, dto.StopAddress, dto.Latitude, dto.Longitude, dto.SequenceOrder, dto.ScheduledTime,
	).Scan(&stop.ID, &stop.RouteID, &stop.StopName, &stop.Address, &stop.Latitude, &stop.Longitude, &stop.StopOrder, &stop.ScheduledArrivalOffsetMinutes, &stop.CreatedAt, &stop.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteStopCreateFailed, Err: err}
	}
	return &stop, nil
}

func (r *TrackingRepository) ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RouteStop, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_route_stops($1, $2, $3)`, tenantID, routeID, scopeUserID)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RouteStopListFailed)
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
	if err := rows.Err(); err != nil {
		return nil, scopedErr(err, trackingErrors.RouteStopListFailed)
	}
	return stops, nil
}

func (r *TrackingRepository) DeleteRouteStop(ctx context.Context, tenantID uuid.UUID, stopID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route_stop($1, $2)`, tenantID, stopID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.RouteStopDeleteFailed, Err: err}
	}
	return nil
}

// ==================== RIDERS CRUD ====================

func (r *TrackingRepository) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_create_rider($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		tenantID, dto.OrganizationID, dto.RiderType, dto.FirstName, dto.LastName, dto.IdentificationNumber,
		dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		dto.GuardianUserID, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail, dto.Address, scopeUserID,
	).Scan(&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RiderCreateFailed)
	}
	return &rider, nil
}

func (r *TrackingRepository) UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_rider($1, $2, $3::VARCHAR(50), $4::VARCHAR(255), $5::VARCHAR(255), $6::VARCHAR(50), $7::UUID, $8::VARCHAR(255), $9::VARCHAR(50), $10::VARCHAR(255), $11::VARCHAR(500), $12::BOOLEAN, $13::UUID)`,
		tenantID, riderID, dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		dto.GuardianUserID, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail,
		dto.Address, dto.IsActive, scopeUserID,
	).Scan(&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RiderUpdateFailed)
	}
	return &rider, nil
}

// ListRiders returns one page of the tenant's riders plus the total the same filters match.
// total_count comes back on every row and stays 0 when the page is empty.
func (r *TrackingRepository) ListRiders(ctx context.Context, tenantID uuid.UUID, query models.ListRidersQuery, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) ([]*models.Rider, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_riders($1::UUID, $2::UUID, $3::VARCHAR, $4::VARCHAR, $5::BOOLEAN, $6::INT, $7::INT, $8::UUID, $9::UUID)`,
		tenantID, query.OrganizationID, query.Search, query.RiderType, query.IsActive, query.Page, query.PageSize, scopeUserID, guardianUserID)
	if err != nil {
		return nil, 0, scopedErr(err, trackingErrors.RiderListFailed)
	}
	defer rows.Close()
	riders := []*models.Rider{}
	var totalCount int64
	for rows.Next() {
		var rider models.Rider
		if err := rows.Scan(&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
			&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
			&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
			&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt, &totalCount); err != nil {
			return nil, 0, err
		}
		riders = append(riders, &rider)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, scopedErr(err, trackingErrors.RiderListFailed)
	}
	return riders, totalCount, nil
}

func (r *TrackingRepository) GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID, scopeUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_rider($1::UUID, $2::UUID, $3::UUID, $4::UUID)`, tenantID, riderID, guardianUserID, scopeUserID).Scan(
		&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RiderNotFound)
	}
	return &rider, nil
}

func (r *TrackingRepository) DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_rider($1, $2, $3)`, tenantID, riderID, scopeUserID).Scan(&deleted)
	if err != nil || !deleted {
		return scopedErr(err, trackingErrors.RiderDeleteFailed)
	}
	return nil
}

// ==================== ASSIGNMENTS CRUD ====================

func (r *TrackingRepository) AssignRider(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.RiderAssignment, error) {
	var assignment models.RiderAssignment
	var status string
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_assign_rider($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::UUID, $6::VARCHAR(50))`,
		tenantID, dto.RiderID, dto.RouteID, dto.PickupStopID, dto.DropoffStopID, "active",
	).Scan(&assignment.ID, &assignment.RiderID, &assignment.RouteID, &assignment.PickupStopID, &assignment.DropoffStopID, &status, &assignment.CreatedAt, &assignment.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "rider.not-found":
				return nil, &trackingErrors.TrackingError{Code: trackingErrors.RiderNotFound, Err: pgErr}
			case "route.not-found":
				return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
			case "assignment.already-exists":
				return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentAlreadyExists, Err: pgErr}
			}
		}
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.AssignmentCreateFailed, Err: err}
	}
	assignment.IsActive = (status == "active")
	assignment.AssignedAt = assignment.CreatedAt
	return &assignment, nil
}

func (r *TrackingRepository) UnassignRider(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_unassign_rider($1, $2)`, tenantID, assignmentID).Scan(&deleted)
	if err != nil || !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.AssignmentDeleteFailed, Err: err}
	}
	return nil
}

func (r *TrackingRepository) ListRiderAssignments(ctx context.Context, tenantID uuid.UUID, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool, scopeUserID *uuid.UUID) ([]*models.RiderAssignment, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_rider_assignments($1::UUID, $2::UUID, $3::UUID, $4::BOOLEAN, $5::UUID)`, tenantID, riderID, routeID, isActive, scopeUserID)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.AssignmentListFailed)
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
	if err := rows.Err(); err != nil {
		return nil, scopedErr(err, trackingErrors.AssignmentListFailed)
	}
	return assignments, nil
}

// ==================== ORGANIZATIONS CRUD ====================

func (r *TrackingRepository) CreateOrganization(ctx context.Context, tenantID uuid.UUID, dto *models.CreateOrganizationDto) (*models.Organization, error) {
	var org models.Organization
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_organization(
			p_tenant_id := $1,
			p_kind      := $2,
			p_name      := $3,
			p_timezone  := $4,
			p_is_active := $5
		)
	`, tenantID, dto.Kind, dto.Name, dto.Timezone, dto.IsActive,
	).Scan(&org.ID, &org.TenantID, &org.Kind, &org.Name, &org.Timezone, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.OrganizationCreateFailed, Err: err}
	}
	return &org, nil
}

func (r *TrackingRepository) UpdateOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, dto *models.UpdateOrganizationDto) (*models.Organization, error) {
	var org models.Organization
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_organization(
			p_tenant_id       := $1,
			p_organization_id := $2,
			p_kind            := $3,
			p_name            := $4,
			p_timezone        := $5,
			p_is_active       := $6
		)
	`, tenantID, organizationID, dto.Kind, dto.Name, dto.Timezone, dto.IsActive,
	).Scan(&org.ID, &org.TenantID, &org.Kind, &org.Name, &org.Timezone, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, mapOrganizationError(err, trackingErrors.OrganizationUpdateFailed)
	}
	return &org, nil
}

// ListOrganizations returns one page of the tenant's organizations plus the total the same filters
// match. total_count comes back on every row and stays 0 when the page is empty.
func (r *TrackingRepository) ListOrganizations(ctx context.Context, tenantID uuid.UUID, query models.ListOrganizationsQuery) ([]*models.Organization, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_organizations($1::UUID, $2::VARCHAR, $3::VARCHAR, $4::BOOLEAN, $5::INT, $6::INT)`,
		tenantID, query.Search, query.Kind, query.IsActive, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.OrganizationListFailed, Err: err}
	}
	defer rows.Close()
	organizations := []*models.Organization{}
	var totalCount int64
	for rows.Next() {
		var org models.Organization
		if err := rows.Scan(&org.ID, &org.TenantID, &org.Kind, &org.Name, &org.Timezone, &org.IsActive,
			&org.CreatedAt, &org.UpdatedAt, &totalCount); err != nil {
			return nil, 0, err
		}
		organizations = append(organizations, &org)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return organizations, totalCount, nil
}

func (r *TrackingRepository) GetOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_organization($1, $2)`, tenantID, organizationID).Scan(
		&org.ID, &org.TenantID, &org.Kind, &org.Name, &org.Timezone, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: err}
	}
	return &org, nil
}

func (r *TrackingRepository) DeleteOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_organization($1, $2)`, tenantID, organizationID).Scan(&deleted)
	if err != nil {
		return mapOrganizationError(err, trackingErrors.OrganizationDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationDeleteFailed}
	}
	return nil
}

// ==================== ORGANIZATION MEMBERS ====================

func (r *TrackingRepository) ListOrganizationMembers(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) ([]*models.OrganizationMember, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_organization_members($1, $2)`, tenantID, organizationID)
	if err != nil {
		return nil, mapOrganizationError(err, trackingErrors.OrganizationMemberListFailed)
	}
	defer rows.Close()
	members := []*models.OrganizationMember{}
	for rows.Next() {
		var member models.OrganizationMember
		if err := rows.Scan(&member.UserID, &member.Email, &member.FirstName, &member.LastName, &member.Role); err != nil {
			return nil, err
		}
		members = append(members, &member)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

// UpsertOrganizationMember adds the member or changes the role of one already there — the SP settles
// which, so the caller needs no branch.
func (r *TrackingRepository) UpsertOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID, role string) (*models.OrganizationMember, error) {
	var member models.OrganizationMember
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_upsert_organization_member(
			p_tenant_id       := $1,
			p_organization_id := $2,
			p_user_id         := $3,
			p_role            := $4
		)
	`, tenantID, organizationID, userID, role,
	).Scan(&member.UserID, &member.Email, &member.FirstName, &member.LastName, &member.Role)
	if err != nil {
		return nil, mapOrganizationError(err, trackingErrors.OrganizationMemberSaveFailed)
	}
	return &member, nil
}

func (r *TrackingRepository) DeleteOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_organization_member($1, $2, $3)`, tenantID, organizationID, userID).Scan(&deleted)
	if err != nil {
		return mapOrganizationError(err, trackingErrors.OrganizationMemberSaveFailed)
	}
	return nil
}

// ==================== DRIVERS CRUD ====================

// driverColumns is the one scan order every driver read shares — list, single and write alike — so
// adding a column to the SPs is one edit here, not five.
func scanDriver(d *models.Driver) []any {
	return []any{
		&d.ID, &d.TenantID, &d.CompanyID, &d.CompanyName, &d.UserID, &d.UserEmail,
		&d.FirstName, &d.LastName, &d.Phone, &d.LicenseNumber, &d.LicenseClass,
		&d.LicenseExpiresOn, &d.Status, &d.CreatedAt, &d.UpdatedAt,
	}
}

// scanDriverRow is scanDriver plus the two columns only the list SP resolves: whether an expired
// blocking document stands against the driver (TRACK-016 D2) and the page's total.
func scanDriverRow(d *models.Driver, total *int64) []any {
	return []any{
		&d.ID, &d.TenantID, &d.CompanyID, &d.CompanyName, &d.UserID, &d.UserEmail,
		&d.FirstName, &d.LastName, &d.Phone, &d.LicenseNumber, &d.LicenseClass,
		&d.LicenseExpiresOn, &d.Status, &d.ServiceBlocked, &d.CreatedAt, &d.UpdatedAt, total,
	}
}

func (r *TrackingRepository) CreateDriver(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDriverDto) (*models.Driver, error) {
	var driver models.Driver
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_driver(
			p_tenant_id          := $1,
			p_company_id         := $2,
			p_user_id            := $3,
			p_first_name         := $4,
			p_last_name          := $5,
			p_phone              := $6,
			p_license_number     := $7,
			p_license_class      := $8,
			p_license_expires_on := $9,
			p_status             := $10
		)
	`, tenantID, dto.CompanyID, dto.UserID, dto.FirstName, dto.LastName, dto.Phone,
		dto.LicenseNumber, dto.LicenseClass, dto.LicenseExpiresOn, dto.Status,
	).Scan(scanDriver(&driver)...)
	if err != nil {
		return nil, mapDriverError(err, trackingErrors.DriverCreateFailed)
	}
	return &driver, nil
}

func (r *TrackingRepository) UpdateDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID, dto *models.UpdateDriverDto) (*models.Driver, error) {
	var driver models.Driver
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_driver(
			p_tenant_id          := $1,
			p_driver_id          := $2,
			p_user_id            := $3,
			p_clear_user_id      := $4,
			p_first_name         := $5,
			p_last_name          := $6,
			p_phone              := $7,
			p_license_number     := $8,
			p_license_class      := $9,
			p_license_expires_on := $10,
			p_status             := $11
		)
	`, tenantID, driverID, dto.UserID, dto.ClearUserID, dto.FirstName, dto.LastName, dto.Phone,
		dto.LicenseNumber, dto.LicenseClass, dto.LicenseExpiresOn, dto.Status,
	).Scan(scanDriver(&driver)...)
	if err != nil {
		return nil, mapDriverError(err, trackingErrors.DriverUpdateFailed)
	}
	return &driver, nil
}

// ListDrivers returns one page of the tenant's drivers plus the total the same filters match.
// total_count comes back on every row and stays 0 when the page is empty.
func (r *TrackingRepository) ListDrivers(ctx context.Context, tenantID uuid.UUID, query models.ListDriversQuery) ([]*models.Driver, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_drivers($1::UUID, $2::VARCHAR, $3::UUID, $4::VARCHAR, $5::INT, $6::INT)`,
		tenantID, query.Search, query.CompanyID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.DriverListFailed, Err: err}
	}
	defer rows.Close()
	drivers := []*models.Driver{}
	var totalCount int64
	for rows.Next() {
		var driver models.Driver
		if err := rows.Scan(scanDriverRow(&driver, &totalCount)...); err != nil {
			return nil, 0, err
		}
		drivers = append(drivers, &driver)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return drivers, totalCount, nil
}

func (r *TrackingRepository) GetDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) (*models.Driver, error) {
	var driver models.Driver
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_driver($1, $2)`, tenantID, driverID).
		Scan(scanDriver(&driver)...)
	if err != nil {
		return nil, mapDriverError(err, trackingErrors.DriverNotFound)
	}
	return &driver, nil
}

func (r *TrackingRepository) DeleteDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_driver($1, $2)`, tenantID, driverID).Scan(&deleted)
	if err != nil {
		return mapDriverError(err, trackingErrors.DriverDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.DriverDeleteFailed}
	}
	return nil
}

// mapDriverError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. The controller maps the module code to a status.
func mapDriverError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "driver.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.DriverNotFound, Err: pgErr}
		case "driver.license-already-exists":
			return &trackingErrors.TrackingError{Code: trackingErrors.DriverLicenseAlreadyExists, Err: pgErr}
		case "driver.user-already-linked":
			return &trackingErrors.TrackingError{Code: trackingErrors.DriverUserAlreadyLinked, Err: pgErr}
		case "driver.has-routes":
			return &trackingErrors.TrackingError{Code: trackingErrors.DriverHasRoutes, Err: pgErr}
		case "company.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.CompanyNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// ==================== DOCUMENT TYPES ====================

func scanDocumentType(t *models.DocumentType) []any {
	return []any{
		&t.ID, &t.TenantID, &t.Code, &t.Name, &t.AppliesTo,
		&t.WarnDaysBefore, &t.BlocksService, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	}
}

func (r *TrackingRepository) CreateDocumentType(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentTypeDto) (*models.DocumentType, error) {
	var docType models.DocumentType
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_document_type(
			p_tenant_id        := $1,
			p_code             := $2,
			p_name             := $3,
			p_applies_to       := $4,
			p_warn_days_before := $5,
			p_blocks_service   := $6,
			p_is_active        := $7
		)
	`, tenantID, dto.Code, dto.Name, dto.AppliesTo, dto.WarnDaysBefore, dto.BlocksService, dto.IsActive,
	).Scan(scanDocumentType(&docType)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentTypeCreateFailed)
	}
	return &docType, nil
}

func (r *TrackingRepository) UpdateDocumentType(ctx context.Context, tenantID uuid.UUID, typeID uuid.UUID, dto *models.UpdateDocumentTypeDto) (*models.DocumentType, error) {
	var docType models.DocumentType
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_document_type(
			p_tenant_id          := $1,
			p_document_type_id   := $2,
			p_code               := $3,
			p_name               := $4,
			p_applies_to         := $5,
			p_warn_days_before   := $6,
			p_blocks_service     := $7,
			p_is_active          := $8
		)
	`, tenantID, typeID, dto.Code, dto.Name, dto.AppliesTo, dto.WarnDaysBefore, dto.BlocksService, dto.IsActive,
	).Scan(scanDocumentType(&docType)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentTypeUpdateFailed)
	}
	return &docType, nil
}

// ListDocumentTypes returns the tenant's whole policy — a handful of rows the document modal needs
// in one go, so it is not paged.
func (r *TrackingRepository) ListDocumentTypes(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentTypesQuery) ([]*models.DocumentType, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_document_types($1::UUID, $2::VARCHAR, $3::BOOLEAN)`,
		tenantID, query.AppliesTo, query.IsActive)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DocumentTypeListFailed, Err: err}
	}
	defer rows.Close()
	types := []*models.DocumentType{}
	for rows.Next() {
		var docType models.DocumentType
		if err := rows.Scan(scanDocumentType(&docType)...); err != nil {
			return nil, err
		}
		types = append(types, &docType)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return types, nil
}

// ==================== COMPLIANCE DOCUMENTS ====================

// scanDocument is the one scan order every document read shares — list, single and write alike.
func scanDocument(d *models.ComplianceDocument) []any {
	return []any{
		&d.ID, &d.TenantID, &d.SubjectType, &d.SubjectID, &d.SubjectName,
		&d.DocumentTypeID, &d.TypeName, &d.Number, &d.IssuedOn, &d.ExpiresOn,
		&d.FileName, &d.FileSize, &d.FileExt, &d.Notes, &d.Status, &d.WarnedAt,
		&d.CreatedAt, &d.UpdatedAt,
	}
}

func (r *TrackingRepository) CreateDocument(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentDto) (*models.ComplianceDocument, error) {
	var doc models.ComplianceDocument
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_document(
			p_tenant_id        := $1,
			p_subject_type     := $2,
			p_subject_id       := $3,
			p_document_type_id := $4,
			p_number           := $5,
			p_issued_on        := $6,
			p_expires_on       := $7,
			p_notes            := $8
		)
	`, tenantID, dto.SubjectType, dto.SubjectID, dto.DocumentTypeID, dto.Number, dto.IssuedOn, dto.ExpiresOn, dto.Notes,
	).Scan(scanDocument(&doc)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentCreateFailed)
	}
	return &doc, nil
}

func (r *TrackingRepository) UpdateDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, dto *models.UpdateDocumentDto) (*models.ComplianceDocument, error) {
	var doc models.ComplianceDocument
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_document(
			p_tenant_id   := $1,
			p_document_id := $2,
			p_number      := $3,
			p_issued_on   := $4,
			p_expires_on  := $5,
			p_notes       := $6
		)
	`, tenantID, documentID, dto.Number, dto.IssuedOn, dto.ExpiresOn, dto.Notes,
	).Scan(scanDocument(&doc)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentUpdateFailed)
	}
	return &doc, nil
}

// ListDocuments returns one page of the tenant's documents plus the total the same filters match.
func (r *TrackingRepository) ListDocuments(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentsQuery) ([]*models.ComplianceDocument, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_documents($1::UUID, $2::VARCHAR, $3::UUID, $4::INT, $5::INT, $6::INT)`,
		tenantID, query.SubjectType, query.SubjectID, query.ExpiringWithinDays, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentListFailed, Err: err}
	}
	defer rows.Close()
	documents := []*models.ComplianceDocument{}
	var totalCount int64
	for rows.Next() {
		var doc models.ComplianceDocument
		if err := rows.Scan(append(scanDocument(&doc), &totalCount)...); err != nil {
			return nil, 0, err
		}
		documents = append(documents, &doc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return documents, totalCount, nil
}

func (r *TrackingRepository) GetDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*models.ComplianceDocument, error) {
	var doc models.ComplianceDocument
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_document($1, $2)`, tenantID, documentID).
		Scan(scanDocument(&doc)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentNotFound)
	}
	return &doc, nil
}

// DeleteDocument removes the row and answers with the extension of the file it had, so the handler
// can delete that file. The filesystem is not the database's to touch.
func (r *TrackingRepository) DeleteDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*string, error) {
	var fileExt *string
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_delete_document($1, $2)`, tenantID, documentID).Scan(&fileExt)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentDeleteFailed)
	}
	return fileExt, nil
}

// SetDocumentFile records the file a document now carries. It runs after the bytes are on disk, so
// a row never claims a file that is not there.
func (r *TrackingRepository) SetDocumentFile(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, name string, size int64, ext string) (*models.ComplianceDocument, error) {
	var doc models.ComplianceDocument
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_set_document_file(
			p_tenant_id   := $1,
			p_document_id := $2,
			p_file_name   := $3,
			p_file_size   := $4,
			p_file_ext    := $5
		)
	`, tenantID, documentID, name, size, ext,
	).Scan(scanDocument(&doc)...)
	if err != nil {
		return nil, mapDocumentError(err, trackingErrors.DocumentFileFailed)
	}
	return &doc, nil
}

// RaiseDocumentAlerts claims every not-yet-warned document inside its type's window and answers with
// what to say about them. Claiming and reporting are one statement, so a document is named exactly
// once however often the job runs (TRACK-016 D1).
func (r *TrackingRepository) RaiseDocumentAlerts(ctx context.Context, tenantID uuid.UUID) ([]*models.DocumentAlert, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_raise_document_alerts($1)`, tenantID)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DocumentListFailed, Err: err}
	}
	defer rows.Close()

	alerts := []*models.DocumentAlert{}
	for rows.Next() {
		var alert models.DocumentAlert
		if err := rows.Scan(&alert.ID, &alert.SubjectType, &alert.SubjectName, &alert.TypeName,
			&alert.Number, &alert.ExpiresOn, &alert.DaysLeft); err != nil {
			return nil, err
		}
		alerts = append(alerts, &alert)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return alerts, nil
}

// mapDocumentError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. The controller maps the module code to a status.
func mapDocumentError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "document.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.DocumentNotFound, Err: pgErr}
		case "document.subject-not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.DocumentSubjectNotFound, Err: pgErr}
		case "document.type-not-applicable":
			return &trackingErrors.TrackingError{Code: trackingErrors.DocumentTypeNotApplicable, Err: pgErr}
		case "document-type.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.DocumentTypeNotFound, Err: pgErr}
		case "document-type.code-already-exists":
			return &trackingErrors.TrackingError{Code: trackingErrors.DocumentTypeCodeAlreadyExists, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// mapOrganizationError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. The controller maps the module code to a status.
func mapOrganizationError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "organization.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: pgErr}
		case "organization.has-riders":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationHasRiders, Err: pgErr}
		case "organization-member.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationMemberNotFound, Err: pgErr}
		case "organization-member.user-not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationMemberUserNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}
