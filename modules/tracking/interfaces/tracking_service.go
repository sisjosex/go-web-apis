package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// TrackingService defines the interface for tracking business logic
type TrackingService interface {
	// Location operations (no tenant scope — GPS device data)
	UpdateVehicleLocation(ctx context.Context, dto *models.UpdateLocationDto) (*models.CurrentLocationResponse, error)
	GetVehicleCurrentLocation(ctx context.Context, vehicleID uuid.UUID) (*models.CurrentLocationResponse, error)

	// Event operations

	// Status queries
	GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.RouteRealtimeStatusResponse, error)
	GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) (*models.RiderStatusResponse, error)

	// Alert operations
	CreateRouteAlert(ctx context.Context, tenantID uuid.UUID, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error)

	// Companies CRUD
	CreateCompany(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCompanyDto) (*models.TransportCompany, error)
	UpdateCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, dto *models.UpdateCompanyDto) (*models.TransportCompany, error)
	ListCompanies(ctx context.Context, tenantID uuid.UUID, query models.ListCompaniesQuery) (*models.ListCompaniesResponse, error)
	GetCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) (*models.TransportCompany, error)
	DeleteCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) error

	// Vehicles CRUD
	CreateVehicle(ctx context.Context, tenantID uuid.UUID, dto *models.CreateVehicleDto) (*models.Vehicle, error)
	UpdateVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error)
	ListVehicles(ctx context.Context, tenantID uuid.UUID, query models.ListVehiclesQuery) (*models.ListVehiclesResponse, error)
	GetVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) (*models.Vehicle, error)
	DeleteVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID) error

	// Routes CRUD
	CreateRoute(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteDto) (*models.Route, error)
	UpdateRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error)
	ListRoutes(ctx context.Context, tenantID uuid.UUID, query models.ListRoutesQuery, scopeUserID *uuid.UUID) (*models.ListRoutesResponse, error)
	GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.Route, error)
	DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error

	// Stop places CRUD (TRACK-007)
	CreateStopPlace(ctx context.Context, tenantID uuid.UUID, dto *models.CreateStopPlaceDto) (*models.StopPlace, error)
	UpdateStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID, dto *models.UpdateStopPlaceDto) (*models.StopPlace, error)
	ListStopPlaces(ctx context.Context, tenantID uuid.UUID, query models.ListStopPlacesQuery, latitude, longitude *float64) (*models.ListStopPlacesResponse, error)
	GetStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) (*models.StopPlace, error)
	DeleteStopPlace(ctx context.Context, tenantID uuid.UUID, stopPlaceID uuid.UUID) error

	// Route versions (TRACK-007)
	ListRouteVersions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteVersion, error)
	CreateRouteVersion(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteVersionDto, createdBy *uuid.UUID) (*models.RouteVersion, error)
	ReplaceRouteVersionStops(ctx context.Context, tenantID uuid.UUID, versionID uuid.UUID, dto *models.ReplaceRouteVersionStopsDto) ([]*models.RouteStop, error)

	// Route schedules and calendars (TRACK-018)
	ListRouteSchedules(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteSchedule, error)
	CreateRouteSchedule(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteScheduleDto) (*models.RouteSchedule, error)
	UpdateRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.UpdateRouteScheduleDto) (*models.RouteSchedule, error)
	DeleteRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID) error
	SplitRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.SplitRouteScheduleDto) (*models.SplitRouteScheduleResponse, error)
	CreateCalendar(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCalendarDto) (*models.Calendar, error)
	UpdateCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dto *models.UpdateCalendarDto) (*models.Calendar, error)
	DeleteCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID) error
	ListCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, from, to *string) ([]*models.CalendarDate, error)
	ReplaceCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dates []models.CalendarDateDto) ([]*models.CalendarDate, error)
	ListCalendars(ctx context.Context, tenantID uuid.UUID, query models.ListCalendarsQuery) (*models.ListCalendarsResponse, error)

	// Route exceptions and the preview (TRACK-018)
	ListRouteExceptions(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dateFrom, dateTo *string) ([]*models.RouteException, error)
	CreateRouteException(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteExceptionDto, createdBy *uuid.UUID) (*models.RouteException, error)
	DeleteRouteException(ctx context.Context, tenantID uuid.UUID, exceptionID uuid.UUID) error
	PreviewRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, from, to string) ([]*models.RoutePreviewDay, error)

	// Route Stops — the list a route runs on one date, read from its version (TRACK-007 D1)
	ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, date *string, scopeUserID *uuid.UUID) ([]*models.RouteStop, error)

	// Riders CRUD
	CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error)
	UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error)
	ListRiders(ctx context.Context, tenantID uuid.UUID, query models.ListRidersQuery, scopeUserID *uuid.UUID, guardianUserID *uuid.UUID) (*models.ListRidersResponse, error)
	GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID, scopeUserID *uuid.UUID) (*models.Rider, error)
	DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID) error

	// Rider route assignments (TRACK-009)
	CreateAssignment(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.AssignmentResponse, error)
	CreateAssignments(ctx context.Context, tenantID uuid.UUID, dtos []models.AssignRiderDto) (*models.BulkAssignmentResponse, error)
	UpdateAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID, dto *models.UpdateAssignmentDto) (*models.AssignmentResponse, error)
	DeleteAssignment(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error
	ListAssignments(ctx context.Context, tenantID uuid.UUID, query models.ListAssignmentsQuery, scopeUserID *uuid.UUID) (*models.ListAssignmentsResponse, error)

	// Organizations CRUD
	CreateOrganization(ctx context.Context, tenantID uuid.UUID, dto *models.CreateOrganizationDto) (*models.Organization, error)
	UpdateOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, dto *models.UpdateOrganizationDto) (*models.Organization, error)
	ListOrganizations(ctx context.Context, tenantID uuid.UUID, query models.ListOrganizationsQuery) (*models.ListOrganizationsResponse, error)
	GetOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) (*models.Organization, error)
	DeleteOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) error

	// Drivers CRUD (TRACK-006)
	CreateDriver(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDriverDto) (*models.Driver, error)
	UpdateDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID, dto *models.UpdateDriverDto) (*models.Driver, error)
	ListDrivers(ctx context.Context, tenantID uuid.UUID, query models.ListDriversQuery) (*models.ListDriversResponse, error)
	GetDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) (*models.Driver, error)
	DeleteDriver(ctx context.Context, tenantID uuid.UUID, driverID uuid.UUID) error

	// Document types and compliance documents (TRACK-016)
	CreateDocumentType(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentTypeDto) (*models.DocumentType, error)
	UpdateDocumentType(ctx context.Context, tenantID uuid.UUID, typeID uuid.UUID, dto *models.UpdateDocumentTypeDto) (*models.DocumentType, error)
	ListDocumentTypes(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentTypesQuery) ([]*models.DocumentType, error)
	CreateDocument(ctx context.Context, tenantID uuid.UUID, dto *models.CreateDocumentDto) (*models.ComplianceDocument, error)
	UpdateDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, dto *models.UpdateDocumentDto) (*models.ComplianceDocument, error)
	ListDocuments(ctx context.Context, tenantID uuid.UUID, query models.ListDocumentsQuery) (*models.ListDocumentsResponse, error)
	GetDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*models.ComplianceDocument, error)
	DeleteDocument(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID) (*string, error)
	SetDocumentFile(ctx context.Context, tenantID uuid.UUID, documentID uuid.UUID, name string, size int64, ext string) (*models.ComplianceDocument, error)
	RaiseDocumentAlerts(ctx context.Context, tenantID uuid.UUID) ([]*models.DocumentAlert, error)

	// Organization members
	ListOrganizationMembers(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) ([]*models.OrganizationMember, error)
	UpsertOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID, role string) (*models.OrganizationMember, error)
	DeleteOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID) error
}
