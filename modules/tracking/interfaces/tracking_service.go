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
	RecordRideEvent(ctx context.Context, dto *models.RecordEventDto, createdBy *uuid.UUID) (*models.EventRecordedResponse, error)

	// Status queries
	GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) (*models.RouteRealtimeStatusResponse, error)
	GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID) (*models.RiderStatusResponse, error)

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
	ListRoutes(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, isActive *bool) ([]*models.Route, error)
	GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) (*models.Route, error)
	DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error

	// Route Stops CRUD
	CreateRouteStop(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteStopDto) (*models.RouteStop, error)
	ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteStop, error)
	DeleteRouteStop(ctx context.Context, tenantID uuid.UUID, stopID uuid.UUID) error

	// Riders CRUD
	CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto) (*models.Rider, error)
	UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto) (*models.Rider, error)
	ListRiders(ctx context.Context, tenantID uuid.UUID, query models.ListRidersQuery) (*models.ListRidersResponse, error)
	GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error)
	DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID) error

	// Assignments CRUD
	AssignRider(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.RiderAssignment, error)
	UnassignRider(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error
	ListRiderAssignments(ctx context.Context, tenantID uuid.UUID, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error)

	// Organizations CRUD
	CreateOrganization(ctx context.Context, tenantID uuid.UUID, dto *models.CreateOrganizationDto) (*models.Organization, error)
	UpdateOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, dto *models.UpdateOrganizationDto) (*models.Organization, error)
	ListOrganizations(ctx context.Context, tenantID uuid.UUID, query models.ListOrganizationsQuery) (*models.ListOrganizationsResponse, error)
	GetOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) (*models.Organization, error)
	DeleteOrganization(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) error

	// Organization members
	ListOrganizationMembers(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID) ([]*models.OrganizationMember, error)
	UpsertOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID, role string) (*models.OrganizationMember, error)
	DeleteOrganizationMember(ctx context.Context, tenantID uuid.UUID, organizationID uuid.UUID, userID uuid.UUID) error
}
