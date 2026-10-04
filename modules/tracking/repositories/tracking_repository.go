package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
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
		return nil, scopedErr(err, trackingErrors.AlertCreateFailed)
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
		if err := rows.Scan(&v.ID, &v.CompanyID, &v.CompanyName, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.ServiceBlocked, &v.CreatedAt, &v.UpdatedAt, &v.DefaultDriverID, &v.DriverName, &totalCount); err != nil {
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
		&v.ID, &v.CompanyID, &v.PlateNumber, &v.VehicleType, &v.Brand, &v.Model, &v.Year, &v.Capacity, &v.GPSDeviceID, &v.Status, &v.CreatedAt, &v.UpdatedAt,
		&v.DefaultDriverID, &v.DriverName)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: err}
	}
	return &v, nil
}

// SetVehicleDriver sets (nil clears) who usually drives a vehicle; its routes without their own
// driver are replanned from today in the same transaction (TRACK-034 D1).
func (r *TrackingRepository) SetVehicleDriver(ctx context.Context, tenantID, vehicleID uuid.UUID, driverID *uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `SELECT tracking.sp_set_vehicle_driver($1, $2, $3)`, tenantID, vehicleID, driverID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "vehicle.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: pgErr}
		case "vehicle.driver-company-mismatch":
			return &trackingErrors.TrackingError{Code: trackingErrors.VehicleDriverCompanyMismatch, Err: pgErr}
		}
	}
	if err != nil {
		return &trackingErrors.TrackingError{Code: trackingErrors.VehicleDriverFailed, Err: err}
	}
	return nil
}

// VehicleScheduleConflicts answers the runs that would overlap if routeID (nil: the vehicle's own
// routes) ran on the vehicle, driverID's other routes included when set (TRACK-034 D2).
func (r *TrackingRepository) VehicleScheduleConflicts(ctx context.Context, tenantID, vehicleID uuid.UUID, routeID, driverID *uuid.UUID) ([]*models.VehicleScheduleConflict, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_vehicle_schedule_conflicts($1, $2, $3, $4)`,
		tenantID, vehicleID, routeID, driverID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == "vehicle.not-found" {
			return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleNotFound, Err: pgErr}
		}
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleConflictsFailed, Err: err}
	}
	defer rows.Close()
	conflicts := []*models.VehicleScheduleConflict{}
	for rows.Next() {
		var c models.VehicleScheduleConflict
		if err := rows.Scan(&c.RouteID, &c.RouteName, &c.Reason, &c.Days, &c.StartTime, &c.EndTime); err != nil {
			return nil, err
		}
		conflicts = append(conflicts, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleConflictsFailed, Err: err}
	}
	return conflicts, nil
}

// VehicleSeatsShort answers the routes the vehicle would run (routeID, or its own) whose riders
// today outnumber its seats (TRACK-038 D3).
func (r *TrackingRepository) VehicleSeatsShort(ctx context.Context, tenantID, vehicleID uuid.UUID, routeID *uuid.UUID) ([]*models.VehicleSeatsShort, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_vehicle_seats_short($1, $2, $3)`, tenantID, vehicleID, routeID)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleConflictsFailed, Err: err}
	}
	defer rows.Close()
	short := []*models.VehicleSeatsShort{}
	for rows.Next() {
		var s models.VehicleSeatsShort
		if err := rows.Scan(&s.RouteID, &s.RouteName, &s.Assigned, &s.Seats); err != nil {
			return nil, err
		}
		short = append(short, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.VehicleConflictsFailed, Err: err}
	}
	return short, nil
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

// scanRoute is the one scan order every route read shares — list, single and write alike — because
// every route SP returns sp_get_route's row (TRACK-002).
func scanRoute(rt *models.Route) []any {
	return []any{
		&rt.ID, &rt.CompanyID, &rt.CompanyName, &rt.VehicleID, &rt.LicensePlate, &rt.DefaultDriverID, &rt.DriverName,
		&rt.RouteName, &rt.RouteCode, &rt.Direction, &rt.OriginAddress, &rt.OriginLat, &rt.OriginLng,
		&rt.DestinationAddress, &rt.DestinationLat, &rt.DestinationLng, &rt.EstimatedDurationMinutes,
		&rt.Timezone, &rt.IsActive, &rt.CreatedAt, &rt.UpdatedAt, &rt.Capacity, &rt.CapacityOverride,
		&rt.OrganizationID, &rt.OrganizationName, &rt.PairedRouteID, &rt.Seats, &rt.AssignedCount,
	}
}

// scanOrganization is the one scan order every organization read and write shares.
func scanOrganization(org *models.Organization) []any {
	return []any{&org.ID, &org.TenantID, &org.Kind, &org.Name, &org.Timezone, &org.IsActive, &org.CreatedAt, &org.UpdatedAt, &org.AbsenceCutoffMin, &org.Latitude, &org.Longitude, &org.Address}
}

// mapRouteError turns the route SPs' own codes into module codes, falling back to fallbackCode.
func mapRouteError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "route.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
		case "company.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.CompanyNotFound, Err: pgErr}
		case "route.company-mismatch":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteCompanyMismatch, Err: pgErr}
		case "route.timezone":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteTimezone, Err: pgErr}
		case "route.code-already-exists":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteCodeAlreadyExists, Err: pgErr}
		case "organization.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: pgErr}
		case "route.destination-required":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteDestinationRequired, Err: pgErr}
		case "route.departure-required":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteDepartureRequired, Err: pgErr}
		case "stop-place.invalid":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceInvalid, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

func (r *TrackingRepository) CreateRoute(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_create_route($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		tenantID, dto.CompanyID, dto.RouteName, dto.OriginAddress, dto.DestinationAddress, dto.Direction,
		dto.VehicleID, dto.DefaultDriverID, dto.Timezone, dto.RouteCode,
		dto.OriginLat, dto.OriginLng, dto.DestinationLat, dto.DestinationLng, dto.EstimatedDurationMinutes, dto.Capacity,
	).Scan(scanRoute(&rt)...)
	if err != nil {
		return nil, mapRouteError(err, trackingErrors.RouteCreateFailed)
	}
	return &rt, nil
}

// CreateRoutePair creates a destination's outbound route and, unless with_return is false, its
// return, in one statement (TRACK-038 D1); it answers both ids, the return's nil without one.
func (r *TrackingRepository) CreateRoutePair(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteDto, createdBy *uuid.UUID) (uuid.UUID, *uuid.UUID, error) {
	stops, err := json.Marshal(dto.Stops)
	if err != nil {
		return uuid.Nil, nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteCreateFailed, Err: err}
	}
	var outbound uuid.UUID
	var inbound *uuid.UUID
	err = r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_create_route_pair($1, $2, $3, $4, $5, $6, $7::TIME, $8::TIME, $9::SMALLINT, $10::JSONB, $11, $12, $13, $14, $15, $16)`,
		tenantID, dto.CompanyID, dto.OrganizationID, dto.RouteName, dto.VehicleID, dto.WithReturn,
		dto.ArrivalTime, dto.DepartureTime, dto.DaysOfWeek, string(stops), dto.Timezone, dto.ReturnRouteName,
		dto.DestinationLat, dto.DestinationLng, nilIfEmpty(dto.DestinationAddress), createdBy,
	).Scan(&outbound, &inbound)
	if err != nil {
		return uuid.Nil, nil, mapRouteError(err, trackingErrors.RouteCreateFailed)
	}
	return outbound, inbound, nil
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (r *TrackingRepository) UpdateRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM tracking.sp_update_route($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		tenantID, routeID, dto.RouteName, dto.RouteCode, dto.Direction, dto.VehicleID, dto.DefaultDriverID, dto.Timezone,
		dto.OriginAddress, dto.OriginLat, dto.OriginLng, dto.DestinationAddress, dto.DestinationLat, dto.DestinationLng,
		dto.EstimatedDurationMinutes, dto.IsActive, dto.Capacity,
	).Scan(scanRoute(&rt)...)
	if err != nil {
		return nil, mapRouteError(err, trackingErrors.RouteUpdateFailed)
	}
	return &rt, nil
}

// ListRoutes returns one page of the tenant's routes plus the total the same filters match
// (TRACK-002 D2); an organization user's page holds only the routes their riders ride.
func (r *TrackingRepository) ListRoutes(ctx context.Context, tenantID uuid.UUID, query models.ListRoutesQuery, scopeUserID *uuid.UUID) ([]*models.Route, int64, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM tracking.sp_list_routes($1::UUID, $2::VARCHAR, $3::UUID, $4::VARCHAR, $5::BOOLEAN, $6::INT, $7::INT, $8::UUID, $9::UUID, $10::UUID)`,
		tenantID, query.Search, query.CompanyID, query.Direction, query.IsActive, query.Page, query.PageSize, scopeUserID, query.VehicleID, query.OrganizationID)
	if err != nil {
		return nil, 0, scopedErr(err, trackingErrors.RouteListFailed)
	}
	defer rows.Close()
	routes := []*models.Route{}
	var totalCount int64
	for rows.Next() {
		var rt models.Route
		if err := rows.Scan(append(scanRoute(&rt), &totalCount)...); err != nil {
			return nil, 0, err
		}
		routes = append(routes, &rt)
	}
	// pgx surfaces an SP exception here, not on Query: without this the refusal would read as an
	// empty list and answer 200.
	if err := rows.Err(); err != nil {
		return nil, 0, scopedErr(err, trackingErrors.RouteListFailed)
	}
	return routes, totalCount, nil
}

func (r *TrackingRepository) GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.Route, error) {
	var rt models.Route
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_route($1, $2, $3)`, tenantID, routeID, scopeUserID).
		Scan(scanRoute(&rt)...)
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

// ==================== STOP PLACES ====================

// scanStopPlace is the one scan order every stop-place read shares — list, single and write alike.
func scanStopPlace(sp *models.StopPlace) []any {
	return []any{
		&sp.ID, &sp.TenantID, &sp.OrganizationID, &sp.Name, &sp.Address,
		&sp.Latitude, &sp.Longitude, &sp.CreatedAt, &sp.UpdatedAt,
	}
}

// mapStopPlaceError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. The controller maps the module code to a status.
func mapStopPlaceError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "stop-place.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceNotFound, Err: pgErr}
		case "stop-place.in-use":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceInUse, Err: pgErr}
		case "organization.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

func (r *TrackingRepository) CreateStopPlace(ctx context.Context, tenantID uuid.UUID, dto *models.CreateStopPlaceDto) (*models.StopPlace, error) {
	var place models.StopPlace
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_stop_place(
			p_tenant_id       := $1,
			p_name            := $2,
			p_address         := $3,
			p_latitude        := $4,
			p_longitude       := $5,
			p_organization_id := $6
		)
	`, tenantID, dto.Name, dto.Address, dto.Latitude, dto.Longitude, dto.OrganizationID,
	).Scan(scanStopPlace(&place)...)
	if err != nil {
		return nil, mapStopPlaceError(err, trackingErrors.StopPlaceCreateFailed)
	}
	return &place, nil
}

func (r *TrackingRepository) UpdateStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID, dto *models.UpdateStopPlaceDto) (*models.StopPlace, error) {
	var place models.StopPlace
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_stop_place(
			p_tenant_id       := $1,
			p_stop_place_id   := $2,
			p_name            := $3,
			p_address         := $4,
			p_latitude        := $5,
			p_longitude       := $6,
			p_organization_id := $7
		)
	`, tenantID, stopPlaceID, dto.Name, dto.Address, dto.Latitude, dto.Longitude, dto.OrganizationID,
	).Scan(scanStopPlace(&place)...)
	if err != nil {
		return nil, mapStopPlaceError(err, trackingErrors.StopPlaceUpdateFailed)
	}
	return &place, nil
}

// ListStopPlaces returns one page of the tenant's stop places plus the total the same filters match.
// latitude and longitude non-nil turn it into a `near` search: every row then carries distance_m and
// the order is nearest first.
func (r *TrackingRepository) ListStopPlaces(ctx context.Context, tenantID uuid.UUID, query models.ListStopPlacesQuery, latitude, longitude *float64) ([]*models.StopPlace, int64, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_stop_places(
			p_tenant_id  := $1,
			p_search     := $2,
			p_latitude   := $3,
			p_longitude  := $4,
			p_radius_m   := $5,
			p_page       := $6,
			p_page_size  := $7
		)
	`, tenantID, query.Search, latitude, longitude, query.RadiusM, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceListFailed, Err: err}
	}
	defer rows.Close()
	places := []*models.StopPlace{}
	var totalCount int64
	for rows.Next() {
		var place models.StopPlace
		if err := rows.Scan(&place.ID, &place.TenantID, &place.OrganizationID, &place.Name, &place.Address,
			&place.Latitude, &place.Longitude, &place.DistanceM, &place.CreatedAt, &place.UpdatedAt, &totalCount); err != nil {
			return nil, 0, err
		}
		places = append(places, &place)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return places, totalCount, nil
}

func (r *TrackingRepository) GetStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) (*models.StopPlace, error) {
	var place models.StopPlace
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_stop_place($1, $2)`, tenantID, stopPlaceID).
		Scan(scanStopPlace(&place)...)
	if err != nil {
		return nil, mapStopPlaceError(err, trackingErrors.StopPlaceNotFound)
	}
	return &place, nil
}

func (r *TrackingRepository) DeleteStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_stop_place($1, $2)`, tenantID, stopPlaceID).Scan(&deleted)
	if err != nil {
		return mapStopPlaceError(err, trackingErrors.StopPlaceDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceDeleteFailed}
	}
	return nil
}

// ==================== ROUTE VERSIONS ====================

// scanRouteVersion is the one scan order every route-version read shares.
func scanRouteVersion(v *models.RouteVersion) []any {
	return []any{
		&v.ID, &v.RouteID, &v.EffectiveFrom, &v.EffectiveTo, &v.StopsCount,
		&v.CreatedBy, &v.CreatedAt, &v.UpdatedAt,
	}
}

// mapRouteVersionError turns the SP's own codes into module codes, falling back to fallbackCode.
func mapRouteVersionError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "route.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
		case "route.version-not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteVersionNotFound, Err: pgErr}
		case "route.version-overlap":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteVersionOverlap, Err: pgErr}
		case "route.version-closed":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteVersionClosed, Err: pgErr}
		case "stop-place.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceNotFound, Err: pgErr}
		case "stop-place.invalid":
			return &trackingErrors.TrackingError{Code: trackingErrors.StopPlaceInvalid, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

func (r *TrackingRepository) ListRouteVersions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteVersion, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_route_versions($1, $2)`, tenantID, routeID)
	if err != nil {
		return nil, mapRouteVersionError(err, trackingErrors.RouteVersionListFailed)
	}
	defer rows.Close()
	versions := []*models.RouteVersion{}
	for rows.Next() {
		var version models.RouteVersion
		if err := rows.Scan(scanRouteVersion(&version)...); err != nil {
			return nil, err
		}
		versions = append(versions, &version)
	}
	if err := rows.Err(); err != nil {
		return nil, mapRouteVersionError(err, trackingErrors.RouteVersionListFailed)
	}
	return versions, nil
}

// CreateRouteVersion publishes the next stop list. The stops travel as one JSONB argument, so the
// whole list is written in the same statement that closes the previous version.
func (r *TrackingRepository) CreateRouteVersion(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteVersionDto, createdBy *uuid.UUID) (*models.RouteVersion, error) {
	stops, err := json.Marshal(dto.Stops)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteVersionCreateFailed, Err: err}
	}
	var version models.RouteVersion
	err = r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_route_version(
			p_tenant_id      := $1,
			p_route_id       := $2,
			p_effective_from := $3,
			p_stops          := $4,
			p_created_by     := $5
		)
	`, tenantID, routeID, time.Time(dto.EffectiveFrom), stops, createdBy,
	).Scan(scanRouteVersion(&version)...)
	if err != nil {
		return nil, mapRouteVersionError(err, trackingErrors.RouteVersionCreateFailed)
	}
	return &version, nil
}

// ReplaceRouteVersionStops swaps the whole list in one round-trip and returns what is stored.
func (r *TrackingRepository) ReplaceRouteVersionStops(ctx context.Context, tenantID uuid.UUID, versionID uuid.UUID, dto *models.ReplaceRouteVersionStopsDto) ([]*models.RouteStop, error) {
	payload, err := json.Marshal(dto.Stops)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.RouteVersionSaveStops, Err: err}
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_replace_route_version_stops(
			p_tenant_id  := $1,
			p_version_id := $2,
			p_stops      := $3
		)
	`, tenantID, versionID, payload)
	if err != nil {
		return nil, mapRouteVersionError(err, trackingErrors.RouteVersionSaveStops)
	}
	defer rows.Close()
	stops := []*models.RouteStop{}
	for rows.Next() {
		var stop models.RouteStop
		if err := rows.Scan(scanRouteStop(&stop)...); err != nil {
			return nil, err
		}
		stops = append(stops, &stop)
	}
	if err := rows.Err(); err != nil {
		return nil, mapRouteVersionError(err, trackingErrors.RouteVersionSaveStops)
	}
	return stops, nil
}

// ==================== ROUTE STOPS ====================

// scanRouteStop is the one scan order every route-stop read shares, so adding a column to the SP is
// one edit here.
func scanRouteStop(stop *models.RouteStop) []any {
	return []any{
		&stop.ID, &stop.RouteID, &stop.VersionID, &stop.StopPlaceID, &stop.StopName, &stop.Address,
		&stop.Latitude, &stop.Longitude, &stop.Sequence, &stop.PlannedOffsetMin, &stop.DwellSec,
		&stop.CreatedAt, &stop.UpdatedAt,
	}
}

// ListRouteStops returns the stops the route runs on date — today when it is nil (TRACK-007 D1).
func (r *TrackingRepository) ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, date *string, scopeUserID *uuid.UUID) ([]*models.RouteStop, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_route_stops(
			p_tenant_id      := $1,
			p_route_id       := $2,
			p_scope_user_id  := $3,
			p_date           := $4
		)
	`, tenantID, routeID, scopeUserID, date)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RouteStopListFailed)
	}
	defer rows.Close()
	stops := []*models.RouteStop{}
	for rows.Next() {
		var stop models.RouteStop
		if err := rows.Scan(scanRouteStop(&stop)...); err != nil {
			return nil, err
		}
		stops = append(stops, &stop)
	}
	if err := rows.Err(); err != nil {
		return nil, scopedErr(err, trackingErrors.RouteStopListFailed)
	}
	return stops, nil
}

// ==================== RIDERS CRUD ====================

// noAccount is what the rider and driver writes pass for the account they used to accept: an account
// is linked only through the access endpoints, which hold it to its level (TRACK-032). NULL keeps
// whatever is stored.
var noAccount *uuid.UUID

func (r *TrackingRepository) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	// One statement: the rider and its guardians' links commit together or not at all (TRACK-037 D2).
	err := r.dbService.QueryRow(ctx, `SELECT r.* FROM tracking.sp_create_rider($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17::DECIMAL, $18::DECIMAL, $19::TEXT) r
		CROSS JOIN LATERAL tracking.fn_rider_guardians_link($1, r.id, $20::JSONB) g`,
		tenantID, dto.OrganizationID, dto.RiderType, dto.FirstName, dto.LastName, dto.IdentificationNumber,
		dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		noAccount, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail, dto.Address, scopeUserID,
		dto.HomeLatitude, dto.HomeLongitude, dto.Notes, linkedGuardians(dto.LinkedGuardians),
	).Scan(&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt,
		&rider.HomeLatitude, &rider.HomeLongitude, &rider.Notes)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == "rider.guardian-invalid" {
			mapped := &trackingErrors.TrackingError{Code: trackingErrors.RiderGuardianInvalid, Err: pgErr}
			if n, convErr := strconv.Atoi(strings.TrimPrefix(pgErr.Detail, "index=")); convErr == nil {
				mapped.Detail = map[string]int{"index": n}
			}
			return nil, mapped
		}
		return nil, scopedErr(err, trackingErrors.RiderCreateFailed)
	}
	return &rider, nil
}

// linkedGuardians is the JSON fn_rider_guardians_link reads; none is an empty list.
func linkedGuardians(raw []byte) string {
	if len(raw) == 0 {
		return "[]"
	}
	return string(raw)
}

func (r *TrackingRepository) UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	var rider models.Rider
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_update_rider($1, $2, $3::VARCHAR(50), $4::VARCHAR(255), $5::VARCHAR(255), $6::VARCHAR(50), $7::UUID, $8::VARCHAR(255), $9::VARCHAR(50), $10::VARCHAR(255), $11::VARCHAR(500), $12::BOOLEAN, $13::UUID, $14::DECIMAL, $15::DECIMAL, $16::TEXT)`,
		tenantID, riderID, dto.Phone, dto.Email, dto.EmergencyContactName, dto.EmergencyContactPhone,
		noAccount, dto.GuardianName, dto.GuardianPhone, dto.GuardianEmail,
		dto.Address, dto.IsActive, scopeUserID, dto.HomeLatitude, dto.HomeLongitude, dto.Notes,
	).Scan(&rider.ID, &rider.OrganizationID, &rider.RiderType, &rider.FirstName, &rider.LastName,
		&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
		&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt,
		&rider.HomeLatitude, &rider.HomeLongitude, &rider.Notes)
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
		if err := rows.Scan(&rider.ID, &rider.OrganizationID, &rider.OrganizationName, &rider.OrganizationKind, &rider.RiderType, &rider.FirstName, &rider.LastName,
			&rider.IdentificationNumber, &rider.Phone, &rider.Email, &rider.EmergencyContactName,
			&rider.EmergencyContactPhone, &rider.GuardianUserID, &rider.GuardianName, &rider.GuardianPhone,
			&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt,
			&rider.HomeLatitude, &rider.HomeLongitude, &rider.Notes, &totalCount); err != nil {
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
		&rider.GuardianEmail, &rider.Address, &rider.IsActive, &rider.CreatedAt, &rider.UpdatedAt,
		&rider.HomeLatitude, &rider.HomeLongitude, &rider.Notes)
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

// ==================== ORGANIZATIONS CRUD ====================

func (r *TrackingRepository) CreateOrganization(ctx context.Context, tenantID uuid.UUID, dto *models.CreateOrganizationDto) (*models.Organization, error) {
	var org models.Organization
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_organization(
			p_tenant_id := $1,
			p_kind      := $2,
			p_name      := $3,
			p_timezone  := $4,
			p_is_active := $5,
			p_absence_cutoff_min := $6,
			p_latitude  := $7,
			p_longitude := $8,
			p_address   := $9
		)
	`, tenantID, dto.Kind, dto.Name, dto.Timezone, dto.IsActive, dto.AbsenceCutoffMin, dto.Latitude, dto.Longitude, dto.Address,
	).Scan(scanOrganization(&org)...)
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
			p_is_active       := $6,
			p_absence_cutoff_min := $7,
			p_latitude        := $8,
			p_longitude       := $9,
			p_address         := $10
		)
	`, tenantID, organizationID, dto.Kind, dto.Name, dto.Timezone, dto.IsActive, dto.AbsenceCutoffMin, dto.Latitude, dto.Longitude, dto.Address,
	).Scan(scanOrganization(&org)...)
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
		if err := rows.Scan(append(scanOrganization(&org), &totalCount)...); err != nil {
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
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_organization($1, $2)`, tenantID, organizationID).
		Scan(scanOrganization(&org)...)
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
	`, tenantID, dto.CompanyID, noAccount, dto.FirstName, dto.LastName, dto.Phone,
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
	`, tenantID, driverID, noAccount, false, dto.FirstName, dto.LastName, dto.Phone,
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
