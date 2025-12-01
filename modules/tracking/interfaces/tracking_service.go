package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// TrackingService defines the interface for tracking business logic
type TrackingService interface {
	// Location operations
	UpdateVehicleLocation(ctx context.Context, dto *models.UpdateLocationDto) (*models.CurrentLocationResponse, error)
	GetVehicleCurrentLocation(ctx context.Context, vehicleID uuid.UUID) (*models.CurrentLocationResponse, error)

	// Event operations
	RecordRideEvent(ctx context.Context, dto *models.RecordEventDto, createdBy *uuid.UUID) (*models.EventRecordedResponse, error)

	// Status queries
	GetRouteRealtimeStatus(ctx context.Context, routeID uuid.UUID) (*models.RouteRealtimeStatusResponse, error)
	GetRiderStatus(ctx context.Context, riderID uuid.UUID) (*models.RiderStatusResponse, error)

	// Alert operations
	CreateRouteAlert(ctx context.Context, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error)

	// Companies CRUD
	CreateCompany(ctx context.Context, dto *models.CreateCompanyDto, tenantID *uuid.UUID) (*models.TransportCompany, error)
	UpdateCompany(ctx context.Context, companyID uuid.UUID, dto *models.UpdateCompanyDto, userTenantID *uuid.UUID) (*models.TransportCompany, error)
	ListCompanies(ctx context.Context, userTenantID *uuid.UUID, companyType *string, isActive *bool) ([]*models.TransportCompany, error)
	GetCompany(ctx context.Context, companyID uuid.UUID, userTenantID *uuid.UUID) (*models.TransportCompany, error)
	DeleteCompany(ctx context.Context, companyID uuid.UUID, userTenantID *uuid.UUID) error

	// Vehicles CRUD
	CreateVehicle(ctx context.Context, dto *models.CreateVehicleDto) (*models.Vehicle, error)
	UpdateVehicle(ctx context.Context, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error)
	ListVehicles(ctx context.Context, companyID *uuid.UUID, vehicleType *string, isActive *bool) ([]*models.Vehicle, error)
	GetVehicle(ctx context.Context, vehicleID uuid.UUID) (*models.Vehicle, error)
	DeleteVehicle(ctx context.Context, vehicleID uuid.UUID) error

	// Routes CRUD
	CreateRoute(ctx context.Context, dto *models.CreateRouteDto) (*models.Route, error)
	UpdateRoute(ctx context.Context, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error)
	ListRoutes(ctx context.Context, companyID *uuid.UUID, isActive *bool) ([]*models.Route, error)
	GetRoute(ctx context.Context, routeID uuid.UUID) (*models.Route, error)
	DeleteRoute(ctx context.Context, routeID uuid.UUID) error

	// Route Stops CRUD
	CreateRouteStop(ctx context.Context, dto *models.CreateRouteStopDto) (*models.RouteStop, error)
	ListRouteStops(ctx context.Context, routeID uuid.UUID) ([]*models.RouteStop, error)
	DeleteRouteStop(ctx context.Context, stopID uuid.UUID) error

	// Riders CRUD
	CreateRider(ctx context.Context, dto *models.CreateRiderDto) (*models.Rider, error)
	UpdateRider(ctx context.Context, riderID uuid.UUID, dto *models.UpdateRiderDto) (*models.Rider, error)
	ListRiders(ctx context.Context, companyID *uuid.UUID, guardianUserID *uuid.UUID, riderType *string, isActive *bool) ([]*models.Rider, error)
	GetRider(ctx context.Context, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error)
	DeleteRider(ctx context.Context, riderID uuid.UUID) error

	// Assignments CRUD
	AssignRider(ctx context.Context, dto *models.AssignRiderDto) (*models.RiderAssignment, error)
	UnassignRider(ctx context.Context, assignmentID uuid.UUID) error
	ListRiderAssignments(ctx context.Context, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error)

	// Client Access operations
	GrantClientAccess(ctx context.Context, companyID uuid.UUID, dto *models.GrantClientAccessDto, userID uuid.UUID, userTenantID uuid.UUID) (*models.CompanyClientAccess, error)
	RevokeClientAccess(ctx context.Context, companyID uuid.UUID, clientTenantID uuid.UUID, userID uuid.UUID, userTenantID uuid.UUID) error
	ListCompanyClients(ctx context.Context, companyID uuid.UUID, userTenantID uuid.UUID) ([]*models.CompanyClientAccess, error)
}
