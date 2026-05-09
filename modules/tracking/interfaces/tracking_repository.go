package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// TrackingRepository defines the interface for tracking data access
type TrackingRepository interface {
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
	ListCompanies(ctx context.Context, tenantID uuid.UUID, registrationNumber *string, status *string) ([]*models.TransportCompany, error)
	GetCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) (*models.TransportCompany, error)
	DeleteCompany(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) error

	// Vehicles CRUD
	CreateVehicle(ctx context.Context, tenantID uuid.UUID, dto *models.CreateVehicleDto) (*models.Vehicle, error)
	UpdateVehicle(ctx context.Context, tenantID uuid.UUID, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error)
	ListVehicles(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, vehicleType *string, status *string) ([]*models.Vehicle, error)
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
	ListRiders(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, guardianUserID *uuid.UUID, riderType *string, isActive *bool) ([]*models.Rider, error)
	GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error)
	DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID) error

	// Assignments CRUD
	AssignRider(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.RiderAssignment, error)
	UnassignRider(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error
	ListRiderAssignments(ctx context.Context, tenantID uuid.UUID, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error)

	// Client Access Management
	GrantClientAccess(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, dto *models.GrantClientAccessDto, grantedBy uuid.UUID) (*models.CompanyClientAccess, error)
	RevokeClientAccess(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, clientTenantID uuid.UUID, revokedBy uuid.UUID) error
	ListCompanyClients(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) ([]*models.CompanyClientAccess, error)
}
