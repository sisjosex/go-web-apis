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

// RecordRideEvent records a boarding/arrival event
func (s *TrackingService) RecordRideEvent(ctx context.Context, dto *models.RecordEventDto, createdBy *uuid.UUID) (*models.EventRecordedResponse, error) {
	return s.trackingRepo.RecordRideEvent(ctx, dto, createdBy)
}

// GetRouteRealtimeStatus gets comprehensive route status
func (s *TrackingService) GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	return s.trackingRepo.GetRouteRealtimeStatus(ctx, tenantID, routeID)
}

// GetRiderStatus gets rider status for guardian view
func (s *TrackingService) GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID) (*models.RiderStatusResponse, error) {
	return s.trackingRepo.GetRiderStatus(ctx, tenantID, riderID)
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

func (s *TrackingService) ListCompanies(ctx context.Context, tenantID uuid.UUID, registrationNumber *string, status *string) ([]*models.TransportCompany, error) {
	return s.trackingRepo.ListCompanies(ctx, tenantID, registrationNumber, status)
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

func (s *TrackingService) ListVehicles(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, vehicleType *string, status *string) ([]*models.Vehicle, error) {
	return s.trackingRepo.ListVehicles(ctx, tenantID, companyID, vehicleType, status)
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

func (s *TrackingService) ListRoutes(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, isActive *bool) ([]*models.Route, error) {
	return s.trackingRepo.ListRoutes(ctx, tenantID, companyID, isActive)
}

func (s *TrackingService) GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) (*models.Route, error) {
	return s.trackingRepo.GetRoute(ctx, tenantID, routeID)
}

func (s *TrackingService) DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error {
	return s.trackingRepo.DeleteRoute(ctx, tenantID, routeID)
}

// ==================== ROUTE STOPS CRUD ====================

func (s *TrackingService) CreateRouteStop(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteStopDto) (*models.RouteStop, error) {
	return s.trackingRepo.CreateRouteStop(ctx, tenantID, dto)
}

func (s *TrackingService) ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteStop, error) {
	return s.trackingRepo.ListRouteStops(ctx, tenantID, routeID)
}

func (s *TrackingService) DeleteRouteStop(ctx context.Context, tenantID uuid.UUID, stopID uuid.UUID) error {
	return s.trackingRepo.DeleteRouteStop(ctx, tenantID, stopID)
}

// ==================== RIDERS CRUD ====================

func (s *TrackingService) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto) (*models.Rider, error) {
	return s.trackingRepo.CreateRider(ctx, tenantID, dto)
}

func (s *TrackingService) UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto) (*models.Rider, error) {
	return s.trackingRepo.UpdateRider(ctx, tenantID, riderID, dto)
}

func (s *TrackingService) ListRiders(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, guardianUserID *uuid.UUID, riderType *string, isActive *bool) ([]*models.Rider, error) {
	return s.trackingRepo.ListRiders(ctx, tenantID, companyID, guardianUserID, riderType, isActive)
}

func (s *TrackingService) GetRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.GetRider(ctx, tenantID, riderID, guardianUserID)
}

func (s *TrackingService) DeleteRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID) error {
	return s.trackingRepo.DeleteRider(ctx, tenantID, riderID)
}

// ==================== ASSIGNMENTS CRUD ====================

func (s *TrackingService) AssignRider(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.RiderAssignment, error) {
	return s.trackingRepo.AssignRider(ctx, tenantID, dto)
}

func (s *TrackingService) UnassignRider(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error {
	return s.trackingRepo.UnassignRider(ctx, tenantID, assignmentID)
}

func (s *TrackingService) ListRiderAssignments(ctx context.Context, tenantID uuid.UUID, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error) {
	return s.trackingRepo.ListRiderAssignments(ctx, tenantID, riderID, routeID, isActive)
}

// ==================== CLIENT ACCESS OPERATIONS ====================

func (s *TrackingService) GrantClientAccess(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, dto *models.GrantClientAccessDto, grantedBy uuid.UUID) (*models.CompanyClientAccess, error) {
	return s.trackingRepo.GrantClientAccess(ctx, tenantID, companyID, dto, grantedBy)
}

func (s *TrackingService) RevokeClientAccess(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID, clientTenantID uuid.UUID, revokedBy uuid.UUID) error {
	return s.trackingRepo.RevokeClientAccess(ctx, tenantID, companyID, clientTenantID, revokedBy)
}

func (s *TrackingService) ListCompanyClients(ctx context.Context, tenantID uuid.UUID, companyID uuid.UUID) ([]*models.CompanyClientAccess, error) {
	return s.trackingRepo.ListCompanyClients(ctx, tenantID, companyID)
}
