package services

import (
	"context"

	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

type TrackingService struct {
	trackingRepo interfaces.TrackingRepository
}

func NewTrackingService(trackingRepo interfaces.TrackingRepository) *TrackingService {
	return &TrackingService{
		trackingRepo: trackingRepo,
	}
}

// UpdateVehicleLocation updates GPS location for a vehicle
func (s *TrackingService) UpdateVehicleLocation(ctx context.Context, dto *models.UpdateLocationDto) (*models.CurrentLocationResponse, error) {
	return s.trackingRepo.UpdateVehicleLocation(ctx, dto)
}

// GetVehicleCurrentLocation retrieves the most recent location
func (s *TrackingService) GetVehicleCurrentLocation(ctx context.Context, vehicleID uuid.UUID) (*models.CurrentLocationResponse, error) {
	return s.trackingRepo.GetVehicleCurrentLocation(ctx, vehicleID)
}

// GetRouteRealtimeStatus gets comprehensive route status
func (s *TrackingService) GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	return s.trackingRepo.GetRouteRealtimeStatus(ctx, tenantID, routeID, scopeUserID)
}

// GetRiderStatus gets rider status for guardian view
func (s *TrackingService) GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) (*models.RiderStatusResponse, error) {
	return s.trackingRepo.GetRiderStatus(ctx, tenantID, riderID, scopeUserID, guardianUserID)
}

// CreateRouteAlert creates a route delay/breakdown alert
func (s *TrackingService) CreateRouteAlert(ctx context.Context, tenantID uuid.UUID, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error) {
	return s.trackingRepo.CreateRouteAlert(ctx, tenantID, dto, createdBy)
}

// ==================== COMPANIES CRUD ====================

func (s *TrackingService) CreateCompany(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCompanyDto) (*models.TransportCompany, error) {
	return s.trackingRepo.CreateCompany(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, dto *models.UpdateCompanyDto) (*models.TransportCompany, error) {
	return s.trackingRepo.UpdateCompany(ctx, tenantID, companyID, dto)
}

func (s *TrackingService) ListCompanies(ctx context.Context, tenantID uuid.UUID, query models.ListCompaniesQuery) (*models.ListCompaniesResponse, error) {
	companies, totalCount, err := s.trackingRepo.ListCompanies(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListCompaniesResponse{
		Companies:  companies,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) (*models.TransportCompany, error) {
	return s.trackingRepo.GetCompany(ctx, tenantID, companyID)
}

func (s *TrackingService) DeleteCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) error {
	return s.trackingRepo.DeleteCompany(ctx, tenantID, companyID)
}

// ==================== VEHICLES CRUD ====================

func (s *TrackingService) CreateVehicle(ctx context.Context, tenantID uuid.UUID, dto *models.CreateVehicleDto) (*models.Vehicle, error) {
	return s.trackingRepo.CreateVehicle(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error) {
	return s.trackingRepo.UpdateVehicle(ctx, tenantID, vehicleID, dto)
}

func (s *TrackingService) ListVehicles(ctx context.Context, tenantID uuid.UUID, query models.ListVehiclesQuery) (*models.ListVehiclesResponse, error) {
	vehicles, totalCount, err := s.trackingRepo.ListVehicles(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListVehiclesResponse{
		Vehicles:   vehicles,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) (*models.Vehicle, error) {
	return s.trackingRepo.GetVehicle(ctx, tenantID, vehicleID)
}

func (s *TrackingService) DeleteVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) error {
	return s.trackingRepo.DeleteVehicle(ctx, tenantID, vehicleID)
}

// ==================== ROUTES CRUD ====================

func (s *TrackingService) CreateRoute(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteDto) (*models.Route, error) {
	return s.trackingRepo.CreateRoute(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error) {
	return s.trackingRepo.UpdateRoute(ctx, tenantID, routeID, dto)
}

func (s *TrackingService) ListRoutes(ctx context.Context, tenantID uuid.UUID, query models.ListRoutesQuery, scopeUserID *uuid.UUID) (*models.ListRoutesResponse, error) {
	routes, totalCount, err := s.trackingRepo.ListRoutes(ctx, tenantID, query, scopeUserID)
	if err != nil {
		return nil, err
	}
	return &models.ListRoutesResponse{
		Routes:     routes,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.Route, error) {
	return s.trackingRepo.GetRoute(ctx, tenantID, routeID, scopeUserID)
}

func (s *TrackingService) DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error {
	return s.trackingRepo.DeleteRoute(ctx, tenantID, routeID)
}

// ==================== ROUTE STOPS CRUD ====================

// ==================== STOP PLACES ====================

func (s *TrackingService) CreateStopPlace(ctx context.Context, tenantID uuid.UUID, dto *models.CreateStopPlaceDto) (*models.StopPlace, error) {
	return s.trackingRepo.CreateStopPlace(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID, dto *models.UpdateStopPlaceDto) (*models.StopPlace, error) {
	return s.trackingRepo.UpdateStopPlace(ctx, tenantID, stopPlaceID, dto)
}

func (s *TrackingService) ListStopPlaces(ctx context.Context, tenantID uuid.UUID, query models.ListStopPlacesQuery, latitude, longitude *float64) (*models.ListStopPlacesResponse, error) {
	places, totalCount, err := s.trackingRepo.ListStopPlaces(ctx, tenantID, query, latitude, longitude)
	if err != nil {
		return nil, err
	}
	return &models.ListStopPlacesResponse{
		StopPlaces: places,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) (*models.StopPlace, error) {
	return s.trackingRepo.GetStopPlace(ctx, tenantID, stopPlaceID)
}

func (s *TrackingService) DeleteStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) error {
	return s.trackingRepo.DeleteStopPlace(ctx, tenantID, stopPlaceID)
}

// ==================== ROUTE VERSIONS ====================

func (s *TrackingService) ListRouteVersions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteVersion, error) {
	return s.trackingRepo.ListRouteVersions(ctx, tenantID, routeID)
}

func (s *TrackingService) CreateRouteVersion(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteVersionDto, createdBy *uuid.UUID) (*models.RouteVersion, error) {
	return s.trackingRepo.CreateRouteVersion(ctx, tenantID, routeID, dto, createdBy)
}

func (s *TrackingService) ReplaceRouteVersionStops(ctx context.Context, tenantID uuid.UUID, versionID uuid.UUID, dto *models.ReplaceRouteVersionStopsDto) ([]*models.RouteStop, error) {
	return s.trackingRepo.ReplaceRouteVersionStops(ctx, tenantID, versionID, dto)
}

func (s *TrackingService) ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, date *string, scopeUserID *uuid.UUID) ([]*models.RouteStop, error) {
	return s.trackingRepo.ListRouteStops(ctx, tenantID, routeID, date, scopeUserID)
}

// ==================== RIDERS CRUD ====================

func (s *TrackingService) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.CreateRider(ctx, tenantID, dto, scopeUserID)
}

func (s *TrackingService) UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.UpdateRider(ctx, tenantID, riderID, dto, scopeUserID)
}

func (s *TrackingService) ListRiders(ctx context.Context, tenantID uuid.UUID, query models.ListRidersQuery, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) (*models.ListRidersResponse, error) {
	riders, totalCount, err := s.trackingRepo.ListRiders(ctx, tenantID, query, scopeUserID, guardianUserID)
	if err != nil {
		return nil, err
	}
	return &models.ListRidersResponse{
		Riders:     riders,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID, scopeUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.GetRider(ctx, tenantID, riderID, guardianUserID, scopeUserID)
}

func (s *TrackingService) DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID) error {
	return s.trackingRepo.DeleteRider(ctx, tenantID, riderID, scopeUserID)
}

// ==================== ASSIGNMENTS CRUD ====================

// CreateAssignment is the bulk path with one item, so a single POST and an import share every check.
func (s *TrackingService) CreateAssignment(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.AssignmentResponse, error) {
	result, err := s.trackingRepo.CreateAssignments(ctx, tenantID, []models.AssignRiderDto{*dto})
	if err != nil {
		return nil, err
	}
	return &models.AssignmentResponse{Assignment: result.Assignments[0], Warnings: result.Warnings}, nil
}

func (s *TrackingService) CreateAssignments(ctx context.Context, tenantID uuid.UUID, dtos []models.AssignRiderDto) (*models.BulkAssignmentResponse, error) {
	return s.trackingRepo.CreateAssignments(ctx, tenantID, dtos)
}

func (s *TrackingService) UpdateAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID, dto *models.UpdateAssignmentDto) (*models.AssignmentResponse, error) {
	return s.trackingRepo.UpdateAssignment(ctx, tenantID, assignmentID, dto)
}

func (s *TrackingService) DeleteAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error {
	return s.trackingRepo.DeleteAssignment(ctx, tenantID, assignmentID)
}

func (s *TrackingService) ListAssignments(ctx context.Context, tenantID uuid.UUID, query models.ListAssignmentsQuery, scopeUserID *uuid.UUID) (*models.ListAssignmentsResponse, error) {
	assignments, totalCount, err := s.trackingRepo.ListAssignments(ctx, tenantID, query, scopeUserID)
	if err != nil {
		return nil, err
	}
	return &models.ListAssignmentsResponse{
		Assignments: assignments,
		TotalCount:  totalCount,
		Page:        query.Page,
		PageSize:    query.PageSize,
	}, nil
}

// ==================== ORGANIZATIONS CRUD ====================

func (s *TrackingService) CreateOrganization(ctx context.Context, tenantID uuid.UUID, dto *models.CreateOrganizationDto) (*models.Organization, error) {
	return s.trackingRepo.CreateOrganization(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, dto *models.UpdateOrganizationDto) (*models.Organization, error) {
	return s.trackingRepo.UpdateOrganization(ctx, tenantID, organizationID, dto)
}

func (s *TrackingService) ListOrganizations(ctx context.Context, tenantID uuid.UUID, query models.ListOrganizationsQuery) (*models.ListOrganizationsResponse, error) {
	organizations, totalCount, err := s.trackingRepo.ListOrganizations(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListOrganizationsResponse{
		Organizations: organizations,
		TotalCount:    totalCount,
		Page:          query.Page,
		PageSize:      query.PageSize,
	}, nil
}

func (s *TrackingService) GetOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) (*models.Organization, error) {
	return s.trackingRepo.GetOrganization(ctx, tenantID, organizationID)
}

func (s *TrackingService) DeleteOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) error {
	return s.trackingRepo.DeleteOrganization(ctx, tenantID, organizationID)
}

// ==================== ORGANIZATION MEMBERS ====================

func (s *TrackingService) ListOrganizationMembers(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) ([]*models.OrganizationMember, error) {
	return s.trackingRepo.ListOrganizationMembers(ctx, tenantID, organizationID)
}

func (s *TrackingService) UpsertOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID, role string) (*models.OrganizationMember, error) {
	return s.trackingRepo.UpsertOrganizationMember(ctx, tenantID, organizationID, userID, role)
}

func (s *TrackingService) DeleteOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID) error {
	return s.trackingRepo.DeleteOrganizationMember(ctx, tenantID, organizationID, userID)
}

// ==================== DRIVERS CRUD ====================

func (s *TrackingService) CreateDriver(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDriverDto) (*models.Driver, error) {
	return s.trackingRepo.CreateDriver(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID, dto *models.UpdateDriverDto) (*models.Driver, error) {
	return s.trackingRepo.UpdateDriver(ctx, tenantID, driverID, dto)
}

func (s *TrackingService) ListDrivers(ctx context.Context, tenantID uuid.UUID, query models.ListDriversQuery) (*models.ListDriversResponse, error) {
	drivers, totalCount, err := s.trackingRepo.ListDrivers(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListDriversResponse{
		Drivers:    drivers,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) (*models.Driver, error) {
	return s.trackingRepo.GetDriver(ctx, tenantID, driverID)
}

func (s *TrackingService) DeleteDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) error {
	return s.trackingRepo.DeleteDriver(ctx, tenantID, driverID)
}

// ==================== DOCUMENT TYPES ====================

func (s *TrackingService) CreateDocumentType(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentTypeDto) (*models.DocumentType, error) {
	return s.trackingRepo.CreateDocumentType(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateDocumentType(ctx context.Context, tenantID uuid.UUID, typeID uuid.UUID, dto *models.UpdateDocumentTypeDto) (*models.DocumentType, error) {
	return s.trackingRepo.UpdateDocumentType(ctx, tenantID, typeID, dto)
}

func (s *TrackingService) ListDocumentTypes(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentTypesQuery) ([]*models.DocumentType, error) {
	return s.trackingRepo.ListDocumentTypes(ctx, tenantID, query)
}

// ==================== COMPLIANCE DOCUMENTS ====================

func (s *TrackingService) CreateDocument(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentDto) (*models.ComplianceDocument, error) {
	return s.trackingRepo.CreateDocument(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, dto *models.UpdateDocumentDto) (*models.ComplianceDocument, error) {
	return s.trackingRepo.UpdateDocument(ctx, tenantID, documentID, dto)
}

func (s *TrackingService) ListDocuments(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentsQuery) (*models.ListDocumentsResponse, error) {
	documents, totalCount, err := s.trackingRepo.ListDocuments(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListDocumentsResponse{
		Documents:  documents,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) GetDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*models.ComplianceDocument, error) {
	return s.trackingRepo.GetDocument(ctx, tenantID, documentID)
}

func (s *TrackingService) DeleteDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*string, error) {
	return s.trackingRepo.DeleteDocument(ctx, tenantID, documentID)
}

func (s *TrackingService) SetDocumentFile(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, name string, size int64, ext string) (*models.ComplianceDocument, error) {
	return s.trackingRepo.SetDocumentFile(ctx, tenantID, documentID, name, size, ext)
}

func (s *TrackingService) RaiseDocumentAlerts(ctx context.Context, tenantID uuid.UUID) ([]*models.DocumentAlert, error) {
	return s.trackingRepo.RaiseDocumentAlerts(ctx, tenantID)
}

// ==================== SCHEDULES AND CALENDARS (TRACK-018) ====================

func (s *TrackingService) ListRouteSchedules(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteSchedule, error) {
	return s.trackingRepo.ListRouteSchedules(ctx, tenantID, routeID)
}

func (s *TrackingService) CreateRouteSchedule(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteScheduleDto) (*models.RouteSchedule, error) {
	return s.trackingRepo.CreateRouteSchedule(ctx, tenantID, routeID, dto)
}

func (s *TrackingService) UpdateRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.UpdateRouteScheduleDto) (*models.RouteSchedule, error) {
	return s.trackingRepo.UpdateRouteSchedule(ctx, tenantID, scheduleID, dto)
}

func (s *TrackingService) DeleteRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID) error {
	return s.trackingRepo.DeleteRouteSchedule(ctx, tenantID, scheduleID)
}

func (s *TrackingService) SplitRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.SplitRouteScheduleDto) (*models.SplitRouteScheduleResponse, error) {
	return s.trackingRepo.SplitRouteSchedule(ctx, tenantID, scheduleID, dto)
}

func (s *TrackingService) ListCalendars(ctx context.Context, tenantID uuid.UUID, query models.ListCalendarsQuery) (*models.ListCalendarsResponse, error) {
	calendars, totalCount, err := s.trackingRepo.ListCalendars(ctx, tenantID, query)
	if err != nil {
		return nil, err
	}
	return &models.ListCalendarsResponse{
		Calendars:  calendars,
		TotalCount: totalCount,
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func (s *TrackingService) CreateCalendar(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCalendarDto) (*models.Calendar, error) {
	return s.trackingRepo.CreateCalendar(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dto *models.UpdateCalendarDto) (*models.Calendar, error) {
	return s.trackingRepo.UpdateCalendar(ctx, tenantID, calendarID, dto)
}

func (s *TrackingService) DeleteCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID) error {
	return s.trackingRepo.DeleteCalendar(ctx, tenantID, calendarID)
}

func (s *TrackingService) ListCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, from, to *string) ([]*models.CalendarDate, error) {
	return s.trackingRepo.ListCalendarDates(ctx, tenantID, calendarID, from, to)
}

func (s *TrackingService) ReplaceCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dates []models.CalendarDateDto) ([]*models.CalendarDate, error) {
	return s.trackingRepo.ReplaceCalendarDates(ctx, tenantID, calendarID, dates)
}

// ==================== EXCEPTIONS AND PREVIEW (TRACK-018) ====================

func (s *TrackingService) ListRouteExceptions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dateFrom, dateTo *string) ([]*models.RouteException, error) {
	return s.trackingRepo.ListRouteExceptions(ctx, tenantID, routeID, dateFrom, dateTo)
}

func (s *TrackingService) CreateRouteException(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteExceptionDto, createdBy *uuid.UUID) (*models.RouteException, error) {
	return s.trackingRepo.CreateRouteException(ctx, tenantID, routeID, dto, createdBy)
}

func (s *TrackingService) DeleteRouteException(ctx context.Context, tenantID uuid.UUID, exceptionID uuid.UUID) error {
	return s.trackingRepo.DeleteRouteException(ctx, tenantID, exceptionID)
}

func (s *TrackingService) PreviewRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, from, to string) ([]*models.RoutePreviewDay, error) {
	return s.trackingRepo.PreviewRoute(ctx, tenantID, routeID, from, to)
}
