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
func (s *TrackingService) GetRouteRealtimeStatus(ctx context.Context, routeID uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	return s.trackingRepo.GetRouteRealtimeStatus(ctx, routeID)
}

// GetRiderStatus gets rider status for guardian view
func (s *TrackingService) GetRiderStatus(ctx context.Context, riderID uuid.UUID) (*models.RiderStatusResponse, error) {
	return s.trackingRepo.GetRiderStatus(ctx, riderID)
}

// CreateRouteAlert creates a route delay/breakdown alert
func (s *TrackingService) CreateRouteAlert(ctx context.Context, dto *models.CreateAlertDto, createdBy *uuid.UUID) (*models.AlertCreatedResponse, error) {
	return s.trackingRepo.CreateRouteAlert(ctx, dto, createdBy)
}

// ==================== COMPANIES CRUD ====================

func (s *TrackingService) CreateCompany(ctx context.Context, dto *models.CreateCompanyDto) (*models.TransportCompany, error) {
	return s.trackingRepo.CreateCompany(ctx, dto)
}

func (s *TrackingService) UpdateCompany(ctx context.Context, companyID uuid.UUID, dto *models.UpdateCompanyDto) (*models.TransportCompany, error) {
	return s.trackingRepo.UpdateCompany(ctx, companyID, dto)
}

func (s *TrackingService) ListCompanies(ctx context.Context, registrationNumber *string, status *string) ([]*models.TransportCompany, error) {
	return s.trackingRepo.ListCompanies(ctx, registrationNumber, status)
}

func (s *TrackingService) GetCompany(ctx context.Context, companyID uuid.UUID) (*models.TransportCompany, error) {
	return s.trackingRepo.GetCompany(ctx, companyID)
}

func (s *TrackingService) DeleteCompany(ctx context.Context, companyID uuid.UUID) error {
	return s.trackingRepo.DeleteCompany(ctx, companyID)
}

// ==================== VEHICLES CRUD ====================

func (s *TrackingService) CreateVehicle(ctx context.Context, dto *models.CreateVehicleDto) (*models.Vehicle, error) {
	return s.trackingRepo.CreateVehicle(ctx, dto)
}

func (s *TrackingService) UpdateVehicle(ctx context.Context, vehicleID uuid.UUID, dto *models.UpdateVehicleDto) (*models.Vehicle, error) {
	return s.trackingRepo.UpdateVehicle(ctx, vehicleID, dto)
}

func (s *TrackingService) ListVehicles(ctx context.Context, companyID *uuid.UUID, vehicleType *string, status *string) ([]*models.Vehicle, error) {
	return s.trackingRepo.ListVehicles(ctx, companyID, vehicleType, status)
}

func (s *TrackingService) GetVehicle(ctx context.Context, vehicleID uuid.UUID) (*models.Vehicle, error) {
	return s.trackingRepo.GetVehicle(ctx, vehicleID)
}

func (s *TrackingService) DeleteVehicle(ctx context.Context, vehicleID uuid.UUID) error {
	return s.trackingRepo.DeleteVehicle(ctx, vehicleID)
}

// ==================== ROUTES CRUD ====================

func (s *TrackingService) CreateRoute(ctx context.Context, dto *models.CreateRouteDto) (*models.Route, error) {
	return s.trackingRepo.CreateRoute(ctx, dto)
}

func (s *TrackingService) UpdateRoute(ctx context.Context, routeID uuid.UUID, dto *models.UpdateRouteDto) (*models.Route, error) {
	return s.trackingRepo.UpdateRoute(ctx, routeID, dto)
}

func (s *TrackingService) ListRoutes(ctx context.Context, companyID *uuid.UUID, isActive *bool) ([]*models.Route, error) {
	return s.trackingRepo.ListRoutes(ctx, companyID, isActive)
}

func (s *TrackingService) GetRoute(ctx context.Context, routeID uuid.UUID) (*models.Route, error) {
	return s.trackingRepo.GetRoute(ctx, routeID)
}

func (s *TrackingService) DeleteRoute(ctx context.Context, routeID uuid.UUID) error {
	return s.trackingRepo.DeleteRoute(ctx, routeID)
}

// ==================== ROUTE STOPS CRUD ====================

func (s *TrackingService) CreateRouteStop(ctx context.Context, dto *models.CreateRouteStopDto) (*models.RouteStop, error) {
	return s.trackingRepo.CreateRouteStop(ctx, dto)
}

func (s *TrackingService) ListRouteStops(ctx context.Context, routeID uuid.UUID) ([]*models.RouteStop, error) {
	return s.trackingRepo.ListRouteStops(ctx, routeID)
}

func (s *TrackingService) DeleteRouteStop(ctx context.Context, stopID uuid.UUID) error {
	return s.trackingRepo.DeleteRouteStop(ctx, stopID)
}

// ==================== RIDERS CRUD ====================

func (s *TrackingService) CreateRider(ctx context.Context, dto *models.CreateRiderDto) (*models.Rider, error) {
	return s.trackingRepo.CreateRider(ctx, dto)
}

func (s *TrackingService) UpdateRider(ctx context.Context, riderID uuid.UUID, dto *models.UpdateRiderDto) (*models.Rider, error) {
	return s.trackingRepo.UpdateRider(ctx, riderID, dto)
}

func (s *TrackingService) ListRiders(ctx context.Context, companyID *uuid.UUID, guardianUserID *uuid.UUID, riderType *string, isActive *bool) ([]*models.Rider, error) {
	return s.trackingRepo.ListRiders(ctx, companyID, guardianUserID, riderType, isActive)
}

func (s *TrackingService) GetRider(ctx context.Context, riderID uuid.UUID, guardianUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.GetRider(ctx, riderID, guardianUserID)
}

func (s *TrackingService) DeleteRider(ctx context.Context, riderID uuid.UUID) error {
	return s.trackingRepo.DeleteRider(ctx, riderID)
}

// ==================== ASSIGNMENTS CRUD ====================

func (s *TrackingService) AssignRider(ctx context.Context, dto *models.AssignRiderDto) (*models.RiderAssignment, error) {
	return s.trackingRepo.AssignRider(ctx, dto)
}

func (s *TrackingService) UnassignRider(ctx context.Context, assignmentID uuid.UUID) error {
	return s.trackingRepo.UnassignRider(ctx, assignmentID)
}

func (s *TrackingService) ListRiderAssignments(ctx context.Context, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool) ([]*models.RiderAssignment, error) {
	return s.trackingRepo.ListRiderAssignments(ctx, riderID, routeID, isActive)
}

// ==================== CLIENT ACCESS OPERATIONS ====================

func (s *TrackingService) GrantClientAccess(ctx context.Context, companyID uuid.UUID, dto *models.GrantClientAccessDto, userID uuid.UUID, userTenantID uuid.UUID) (*models.CompanyClientAccess, error) {
	return s.trackingRepo.GrantClientAccess(ctx, companyID, dto, userID, userTenantID)
}

func (s *TrackingService) RevokeClientAccess(ctx context.Context, companyID uuid.UUID, clientTenantID uuid.UUID, userID uuid.UUID, userTenantID uuid.UUID) error {
	return s.trackingRepo.RevokeClientAccess(ctx, companyID, clientTenantID, userID, userTenantID)
}

func (s *TrackingService) ListCompanyClients(ctx context.Context, companyID uuid.UUID, userTenantID uuid.UUID) ([]*models.CompanyClientAccess, error) {
	return s.trackingRepo.ListCompanyClients(ctx, companyID, userTenantID)
}
