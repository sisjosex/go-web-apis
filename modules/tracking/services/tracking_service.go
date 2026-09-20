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
func (s *TrackingService) GetRouteRealtimeStatus(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.RouteRealtimeStatusResponse, error) {
	return s.trackingRepo.GetRouteRealtimeStatus(ctx, tenantID, routeID, scopeUserID)
}

// GetRiderStatus gets rider status for guardian view
func (s *TrackingService) GetRiderStatus(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, scopeUserID *uuid.UUID) (*models.RiderStatusResponse, error) {
	return s.trackingRepo.GetRiderStatus(ctx, tenantID, riderID, scopeUserID)
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

func (s *TrackingService) ListRoutes(ctx context.Context, tenantID uuid.UUID, companyID *uuid.UUID, isActive *bool, scopeUserID *uuid.UUID) ([]*models.Route, error) {
	return s.trackingRepo.ListRoutes(ctx, tenantID, companyID, isActive, scopeUserID)
}

func (s *TrackingService) GetRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) (*models.Route, error) {
	return s.trackingRepo.GetRoute(ctx, tenantID, routeID, scopeUserID)
}

func (s *TrackingService) DeleteRoute(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) error {
	return s.trackingRepo.DeleteRoute(ctx, tenantID, routeID)
}

// ==================== ROUTE STOPS CRUD ====================

func (s *TrackingService) CreateRouteStop(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRouteStopDto) (*models.RouteStop, error) {
	return s.trackingRepo.CreateRouteStop(ctx, tenantID, dto)
}

func (s *TrackingService) ListRouteStops(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RouteStop, error) {
	return s.trackingRepo.ListRouteStops(ctx, tenantID, routeID, scopeUserID)
}

func (s *TrackingService) DeleteRouteStop(ctx context.Context, tenantID uuid.UUID, stopID uuid.UUID) error {
	return s.trackingRepo.DeleteRouteStop(ctx, tenantID, stopID)
}

// ==================== RIDERS CRUD ====================

func (s *TrackingService) CreateRider(ctx context.Context, tenantID uuid.UUID, dto *models.CreateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.CreateRider(ctx, tenantID, dto, scopeUserID)
}

func (s *TrackingService) UpdateRider(ctx context.Context, tenantID uuid.UUID, riderID uuid.UUID, dto *models.UpdateRiderDto, scopeUserID *uuid.UUID) (*models.Rider, error) {
	return s.trackingRepo.UpdateRider(ctx, tenantID, riderID, dto, scopeUserID)
}

func (s *TrackingService) ListRiders(ctx context.Context, tenantID uuid.UUID, query models.ListRidersQuery, scopeUserID *uuid.UUID) (*models.ListRidersResponse, error) {
	riders, totalCount, err := s.trackingRepo.ListRiders(ctx, tenantID, query, scopeUserID)
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

func (s *TrackingService) AssignRider(ctx context.Context, tenantID uuid.UUID, dto *models.AssignRiderDto) (*models.RiderAssignment, error) {
	return s.trackingRepo.AssignRider(ctx, tenantID, dto)
}

func (s *TrackingService) UnassignRider(ctx context.Context, tenantID uuid.UUID, assignmentID uuid.UUID) error {
	return s.trackingRepo.UnassignRider(ctx, tenantID, assignmentID)
}

func (s *TrackingService) ListRiderAssignments(ctx context.Context, tenantID uuid.UUID, riderID *uuid.UUID, routeID *uuid.UUID, isActive *bool, scopeUserID *uuid.UUID) ([]*models.RiderAssignment, error) {
	return s.trackingRepo.ListRiderAssignments(ctx, tenantID, riderID, routeID, isActive, scopeUserID)
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
