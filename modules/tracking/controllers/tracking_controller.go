package controllers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leebenson/conform"

	"josex/web/config"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/limits"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"
	trackingServices "josex/web/modules/tracking/services"
)

type TrackingController struct {
	trackingService interfaces.TrackingService
	// documentFiles keeps compliance document scans in the documents bucket (INFRA-007). It is the
	// controller's and not the service's: the SPs own the rows, the bucket is the handler's to touch.
	documentFiles *trackingServices.DocumentFiles
	// proofFiles signs the links to drop-off proof photos (MOBILE-032).
	proofFiles *trackingServices.ProofFiles
}

func NewTrackingController(trackingService interfaces.TrackingService, documentFiles *trackingServices.DocumentFiles, proofFiles *trackingServices.ProofFiles) *TrackingController {
	return &TrackingController{
		trackingService: trackingService,
		documentFiles:   documentFiles,
		proofFiles:      proofFiles,
	}
}

// getTenantID extracts tenant_id from context if multitenancy is enabled
func (ctrl *TrackingController) getTenantID(c *gin.Context) (*uuid.UUID, error) {
	coreConf := config.ModularAppConfig.Core
	tenancyConf := config.ModularAppConfig.Tenancy

	if !coreConf.IsModuleEnabled("tenancy") || tenancyConf == nil || !tenancyConf.Enabled {
		return nil, nil
	}

	tenantIDVal, exists := c.Get("tenant_id")
	if !exists {
		return nil, errors.New("tenant_id not found in context")
	}

	if tenantIDVal == nil {
		return nil, nil
	}

	if tenantID, ok := tenantIDVal.(uuid.UUID); ok {
		return &tenantID, nil
	}

	if str, ok := tenantIDVal.(string); ok && str != "" {
		parsed, err := uuid.Parse(str)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	}

	return nil, nil
}

// requireTenantID extracts tenant_id and returns 403 if not present
func (ctrl *TrackingController) requireTenantID(c *gin.Context) (uuid.UUID, error) {
	tenantID, err := ctrl.getTenantID(c)
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if tenantID == nil {
		return uuid.Nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
	}
	return *tenantID, nil
}

// scopeUserID is the user a scoped read is narrowed by (TRACK-015 D1). Only an `organization`
// access level carries one: the SPs take it as p_scope_user_id and resolve the memberships
// themselves, one round-trip, so no path list lives here. Every other level — operator, super_admin
// — gets nil and reads the whole tenant, exactly as before.
func (ctrl *TrackingController) scopeUserID(c *gin.Context) *uuid.UUID {
	return userIDAtLevel(c, tenancyModels.RoleOrganization)
}

// guardianUserID is the user a guardian's read is narrowed by (TRACK-017 D2). Only a `portal`
// account carries one: the two reads it may make take it as p_guardian_user_id and resolve the
// rider_contacts rows themselves, in the same round-trip. Every other level — operator,
// organization, super_admin — gets nil and reads exactly as before.
//
// Unlike scopeUserID, an empty scope is not a refusal: a parent the school has not linked yet reads
// an empty list, which is what the app has copy for.
func (ctrl *TrackingController) guardianUserID(c *gin.Context) *uuid.UUID {
	return userIDAtLevel(c, tenancyModels.RolePortal)
}

// userIDAtLevel is the signed-in user when their level in the tenant is role, nil otherwise — what
// scopeUserID and guardianUserID narrow a read by.
func userIDAtLevel(c *gin.Context, role string) *uuid.UUID {
	level, exists := c.Get("tenant_user_role")
	if !exists || level != role {
		return nil
	}
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil
	}
	return &userID
}

// actingUserID is the signed-in user a write is recorded against. It is read from the context the
// auth middleware filled, never from the body.
func (ctrl *TrackingController) actingUserID(c *gin.Context) *uuid.UUID {
	return actingUser(c)
}

func actingUser(c *gin.Context) *uuid.UUID {
	raw, exists := c.Get("user_id")
	if !exists {
		return nil
	}
	if userID, ok := raw.(uuid.UUID); ok {
		return &userID
	}
	if str, ok := raw.(string); ok {
		if userID, err := uuid.Parse(str); err == nil {
			return &userID
		}
	}
	return nil
}

// scopeRefused answers 403 when the caller's access level is organization but they belong to no
// organization in this tenant — a misconfigured account, not an empty result. It reports whether it
// has already written the response.
func scopeRefused(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationScopeDenied {
		return api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// GetVehicleCurrentLocationInput is GetVehicleCurrentLocation's request.
type GetVehicleCurrentLocationInput struct {
	VehicleID string `path:"vehicle_id"`
}

// GetVehicleCurrentLocation is GET /vehicles/{vehicle_id}/location: Get vehicle current location.
func (ctrl *TrackingController) GetVehicleCurrentLocation(ctx context.Context, in *GetVehicleCurrentLocationInput) (*api.Out[*models.CurrentLocationResponse], error) {
	c := api.Gin(ctx)
	vehicleIDStr := in.VehicleID
	vehicleID, err := uuid.Parse(vehicleIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	response, err := ctrl.trackingService.GetVehicleCurrentLocation(c.Request.Context(), vehicleID)
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.VehicleNotFound, trackingErrors.LocationNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}

		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(response)
}

// GetRouteRealtimeStatusInput is GetRouteRealtimeStatus's request.
type GetRouteRealtimeStatusInput struct {
	RouteID string `path:"route_id"`
}

// GetRouteRealtimeStatus is GET /routes/{route_id}/status: Get route real-time status.
func (ctrl *TrackingController) GetRouteRealtimeStatus(ctx context.Context, in *GetRouteRealtimeStatusInput) (*api.Out[*models.RouteRealtimeStatusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeIDStr := in.RouteID
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	response, err := ctrl.trackingService.GetRouteRealtimeStatus(c.Request.Context(), tenantID, routeID, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RouteNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}

		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(response)
}

// GetRiderStatusInput is GetRiderStatus's request.
type GetRiderStatusInput struct {
	RiderID string `path:"rider_id"`
}

// GetRiderStatus is GET /riders/{rider_id}/status: Get rider status (for guardians).
func (ctrl *TrackingController) GetRiderStatus(ctx context.Context, in *GetRiderStatusInput) (*api.Out[*models.RiderStatusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	riderIDStr := in.RiderID
	riderID, err := uuid.Parse(riderIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	response, err := ctrl.trackingService.GetRiderStatus(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RiderNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}

		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(response)
}

// ListGuardianRiderStatusResponse is ListGuardianRiderStatus's answer.
type ListGuardianRiderStatusResponse struct {
	Riders []models.RiderStatusResponse `json:"riders"`
}

// ListGuardianRiderStatus is GET /riders/today: Today's status of each of the guardian's riders.
func (ctrl *TrackingController) ListGuardianRiderStatus(ctx context.Context, _ *api.None) (*api.Out[ListGuardianRiderStatusResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	// A guardian's read only: for anyone else "my riders" is the whole tenant, which no home lists.
	guardian := ctrl.guardianUserID(c)
	if guardian == nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
	}
	riders, err := ctrl.trackingService.ListGuardianRiderStatus(c.Request.Context(), tenantID, *guardian)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(ListGuardianRiderStatusResponse{Riders: riders})
}

// CreateRouteAlertInput is CreateRouteAlert's request.
type CreateRouteAlertInput struct {
	JSON api.Body[models.CreateAlertDto]
}

// CreateRouteAlert is POST /alerts: Create route alert.
func (ctrl *TrackingController) CreateRouteAlert(ctx context.Context, in *CreateRouteAlertInput) (*api.Out[*models.AlertCreatedResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateAlertDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AlertCreateFailed, utils.ExtractValidationError(c, err)))
	}

	conform.Strings(&dto)

	response, err := ctrl.trackingService.CreateRouteAlert(c.Request.Context(), tenantID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RouteNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}

		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(response)
}

// ==================== COMPANIES CRUD ====================

// CreateCompanyInput is CreateCompany's request.
type CreateCompanyInput struct {
	JSON api.Body[models.CreateCompanyDto]
}

// CreateCompany is POST /companies: Create transport company.
func (ctrl *TrackingController) CreateCompany(ctx context.Context, in *CreateCompanyInput) (*api.Out[*models.TransportCompany], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateCompanyDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CompanyCreateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)

	company, err := ctrl.trackingService.CreateCompany(c.Request.Context(), tenantID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(company)
}

// UpdateCompanyInput is UpdateCompany's request.
type UpdateCompanyInput struct {
	CompanyID string `path:"company_id"`
	JSON      api.Body[models.UpdateCompanyDto]
}

// UpdateCompany is PATCH /companies/{company_id}: Update transport company.
func (ctrl *TrackingController) UpdateCompany(ctx context.Context, in *UpdateCompanyInput) (*api.Out[*models.TransportCompany], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	companyID, err := uuid.Parse(in.CompanyID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateCompanyDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CompanyUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)

	company, err := ctrl.trackingService.UpdateCompany(c.Request.Context(), tenantID, companyID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(company)
}

// ListCompaniesInput is ListCompanies's request.
type ListCompaniesInput struct {
	Query api.Query[models.ListCompaniesQuery]
}

// ListCompanies is GET /companies: List transport companies.
func (ctrl *TrackingController) ListCompanies(ctx context.Context, in *ListCompaniesInput) (*api.Out[*models.ListCompaniesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListCompaniesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListCompanies(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetCompanyInput is GetCompany's request.
type GetCompanyInput struct {
	CompanyID string `path:"company_id"`
}

// GetCompany is GET /companies/{company_id}: Get company by ID.
func (ctrl *TrackingController) GetCompany(ctx context.Context, in *GetCompanyInput) (*api.Out[*models.TransportCompany], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	companyID, err := uuid.Parse(in.CompanyID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	company, err := ctrl.trackingService.GetCompany(c.Request.Context(), tenantID, companyID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.CompanyNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(company)
}

// DeleteCompanyInput is DeleteCompany's request.
type DeleteCompanyInput struct {
	CompanyID string `path:"company_id"`
}

// DeleteCompany is DELETE /companies/{company_id}: Delete transport company.
func (ctrl *TrackingController) DeleteCompany(ctx context.Context, in *DeleteCompanyInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	companyID, err := uuid.Parse(in.CompanyID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	err = ctrl.trackingService.DeleteCompany(c.Request.Context(), tenantID, companyID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.CompanyNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			case trackingErrors.CompanyHasVehicles:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== VEHICLES CRUD ====================

// CreateVehicleInput is CreateVehicle's request.
type CreateVehicleInput struct {
	JSON api.Body[models.CreateVehicleDto]
}

// CreateVehicle is POST /vehicles: Create vehicle.
func (ctrl *TrackingController) CreateVehicle(ctx context.Context, in *CreateVehicleInput) (*api.Out[*models.Vehicle], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateVehicleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleCreateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	vehicle, err := ctrl.trackingService.CreateVehicle(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := limits.Refuse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(vehicle)
}

// UpdateVehicleInput is UpdateVehicle's request.
type UpdateVehicleInput struct {
	VehicleID string `path:"vehicle_id"`
	JSON      api.Body[models.UpdateVehicleDto]
}

// UpdateVehicle is PATCH /vehicles/{vehicle_id}: Update vehicle.
func (ctrl *TrackingController) UpdateVehicle(ctx context.Context, in *UpdateVehicleInput) (*api.Out[*models.Vehicle], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateVehicleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	vehicle, err := ctrl.trackingService.UpdateVehicle(c.Request.Context(), tenantID, vehicleID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(vehicle)
}

// ListVehiclesInput is ListVehicles's request.
type ListVehiclesInput struct {
	Query api.Query[models.ListVehiclesQuery]
}

// ListVehicles is GET /vehicles: List vehicles.
func (ctrl *TrackingController) ListVehicles(ctx context.Context, in *ListVehiclesInput) (*api.Out[*models.ListVehiclesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListVehiclesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListVehicles(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetVehicleInput is GetVehicle's request.
type GetVehicleInput struct {
	VehicleID string `path:"vehicle_id"`
}

// GetVehicle is GET /vehicles/{vehicle_id}: Get vehicle by ID.
func (ctrl *TrackingController) GetVehicle(ctx context.Context, in *GetVehicleInput) (*api.Out[*models.Vehicle], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	vehicle, err := ctrl.trackingService.GetVehicle(c.Request.Context(), tenantID, vehicleID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.VehicleNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(vehicle)
}

// DeleteVehicleInput is DeleteVehicle's request.
type DeleteVehicleInput struct {
	VehicleID string `path:"vehicle_id"`
}

// DeleteVehicle is DELETE /vehicles/{vehicle_id}: Delete vehicle.
func (ctrl *TrackingController) DeleteVehicle(ctx context.Context, in *DeleteVehicleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	vehicleID, err := uuid.Parse(in.VehicleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	err = ctrl.trackingService.DeleteVehicle(c.Request.Context(), tenantID, vehicleID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.VehicleNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== ROUTES CRUD ====================

// CreateRouteInput is CreateRoute's request.
type CreateRouteInput struct {
	JSON api.Body[models.CreateRouteDto]
}

// CreateRoute is POST /routes: Create route.
func (ctrl *TrackingController) CreateRoute(ctx context.Context, in *CreateRouteInput) (*api.Out[*models.CreatedRoute], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateRouteDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteCreateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	route, err := ctrl.trackingService.CreateRoute(c.Request.Context(), tenantID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if err := routeErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(route)
}

// CreateRouteReturnInput is CreateRouteReturn's request.
type CreateRouteReturnInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.CreateRouteReturnDto]
}

// CreateRouteReturn is POST /routes/{route_id}/return: Create the return of a route.
func (ctrl *TrackingController) CreateRouteReturn(ctx context.Context, in *CreateRouteReturnInput) (*api.Out[*models.Route], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.CreateRouteReturnDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteDepartureRequired, utils.ExtractValidationError(c, err)))
	}
	route, err := ctrl.trackingService.CreateRouteReturn(c.Request.Context(), tenantID, routeID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if err := routeErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(route)
}

// UpdateRouteInput is UpdateRoute's request.
type UpdateRouteInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.UpdateRouteDto]
}

// UpdateRoute is PATCH /routes/{route_id}: Update route.
func (ctrl *TrackingController) UpdateRoute(ctx context.Context, in *UpdateRouteInput) (*api.Out[*models.Route], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateRouteDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	route, err := ctrl.trackingService.UpdateRoute(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if err := routeErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(route)
}

// ListRoutesInput is ListRoutes's request.
type ListRoutesInput struct {
	Query api.Query[models.ListRoutesQuery]
}

// ListRoutes is GET /routes: List routes.
func (ctrl *TrackingController) ListRoutes(ctx context.Context, in *ListRoutesInput) (*api.Out[*models.ListRoutesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListRoutesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListRoutes(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetRouteInput is GetRoute's request.
type GetRouteInput struct {
	RouteID string `path:"route_id"`
}

// GetRoute is GET /routes/{route_id}: Get route by ID.
func (ctrl *TrackingController) GetRoute(ctx context.Context, in *GetRouteInput) (*api.Out[*models.Route], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	route, err := ctrl.trackingService.GetRoute(c.Request.Context(), tenantID, routeID, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(route)
}

// DeleteRouteInput is DeleteRoute's request.
type DeleteRouteInput struct {
	RouteID string `path:"route_id"`
}

// DeleteRoute is DELETE /routes/{route_id}: Delete route.
func (ctrl *TrackingController) DeleteRoute(ctx context.Context, in *DeleteRouteInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	err = ctrl.trackingService.DeleteRoute(c.Request.Context(), tenantID, routeID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== ROUTE VERSIONS ====================

// routeVersionErrorResponse maps the route-version codes to statuses and reports whether it has
// already written the response.
func routeVersionErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.RouteVersionNotFound, trackingErrors.StopPlaceNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.RouteVersionOverlap, trackingErrors.RouteVersionClosed:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.StopPlaceInvalid:
		return api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// ListRouteVersionsInput is ListRouteVersions's request.
type ListRouteVersionsInput struct {
	RouteID string `path:"route_id"`
}

// ListRouteVersions is GET /routes/{route_id}/versions: List a route's stop lists over time.
func (ctrl *TrackingController) ListRouteVersions(ctx context.Context, in *ListRouteVersionsInput) (*api.Out[[]*models.RouteVersion], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	versions, err := ctrl.trackingService.ListRouteVersions(c.Request.Context(), tenantID, routeID)
	if err != nil {
		if err := routeVersionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(versions)
}

// CreateRouteVersionInput is CreateRouteVersion's request.
type CreateRouteVersionInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.CreateRouteVersionDto]
}

// CreateRouteVersion is POST /routes/{route_id}/versions: Publish a route's next stop list.
func (ctrl *TrackingController) CreateRouteVersion(ctx context.Context, in *CreateRouteVersionInput) (*api.Out[*models.RouteVersion], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.CreateRouteVersionDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteVersionCreateFailed, utils.ExtractValidationError(c, err)))
	}

	version, err := ctrl.trackingService.CreateRouteVersion(c.Request.Context(), tenantID, routeID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if err := routeVersionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(version)
}

// ReplaceRouteVersionStopsInput is ReplaceRouteVersionStops's request.
type ReplaceRouteVersionStopsInput struct {
	RouteID   string `path:"route_id"`
	VersionID string `path:"version_id"`
	JSON      api.Body[models.ReplaceRouteVersionStopsDto]
}

// ReplaceRouteVersionStops is PUT /routes/{route_id}/versions/{version_id}/stops: Replace a route version's whole stop list.
func (ctrl *TrackingController) ReplaceRouteVersionStops(ctx context.Context, in *ReplaceRouteVersionStopsInput) (*api.Out[[]*models.RouteStop], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	versionID, err := uuid.Parse(in.VersionID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.ReplaceRouteVersionStopsDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteVersionSaveStops, utils.ExtractValidationError(c, err)))
	}

	stops, err := ctrl.trackingService.ReplaceRouteVersionStops(c.Request.Context(), tenantID, versionID, &dto)
	if err != nil {
		if err := routeVersionErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(stops)
}

// ==================== STOP PLACES ====================

// stopPlaceErrorResponse maps the stop-place codes to statuses and reports whether it has already
// written the response.
func stopPlaceErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.StopPlaceNotFound, trackingErrors.OrganizationNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.StopPlaceInUse:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// CreateStopPlaceInput is CreateStopPlace's request.
type CreateStopPlaceInput struct {
	JSON api.Body[models.CreateStopPlaceDto]
}

// CreateStopPlace is POST /stop-places: Create stop place.
func (ctrl *TrackingController) CreateStopPlace(ctx context.Context, in *CreateStopPlaceInput) (*api.Out[*models.StopPlace], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateStopPlaceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.StopPlaceCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	place, err := ctrl.trackingService.CreateStopPlace(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := stopPlaceErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(place)
}

// UpdateStopPlaceInput is UpdateStopPlace's request.
type UpdateStopPlaceInput struct {
	StopPlaceID string `path:"stop_place_id"`
	JSON        api.Body[models.UpdateStopPlaceDto]
}

// UpdateStopPlace is PATCH /stop-places/{stop_place_id}: Update stop place.
func (ctrl *TrackingController) UpdateStopPlace(ctx context.Context, in *UpdateStopPlaceInput) (*api.Out[*models.StopPlace], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	stopPlaceID, err := uuid.Parse(in.StopPlaceID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateStopPlaceDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.StopPlaceUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	place, err := ctrl.trackingService.UpdateStopPlace(c.Request.Context(), tenantID, stopPlaceID, &dto)
	if err != nil {
		if err := stopPlaceErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(place)
}

// ListStopPlacesInput is ListStopPlaces's request.
type ListStopPlacesInput struct {
	Query api.Query[models.ListStopPlacesQuery]
}

// ListStopPlaces is GET /stop-places: List stop places.
func (ctrl *TrackingController) ListStopPlaces(ctx context.Context, in *ListStopPlacesInput) (*api.Out[*models.ListStopPlacesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListStopPlacesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)
	latitude, longitude, err := query.NearPoint()
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	result, err := ctrl.trackingService.ListStopPlaces(c.Request.Context(), tenantID, query, latitude, longitude)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetStopPlaceInput is GetStopPlace's request.
type GetStopPlaceInput struct {
	StopPlaceID string `path:"stop_place_id"`
}

// GetStopPlace is GET /stop-places/{stop_place_id}: Get stop place by ID.
func (ctrl *TrackingController) GetStopPlace(ctx context.Context, in *GetStopPlaceInput) (*api.Out[*models.StopPlace], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	stopPlaceID, err := uuid.Parse(in.StopPlaceID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	place, err := ctrl.trackingService.GetStopPlace(c.Request.Context(), tenantID, stopPlaceID)
	if err != nil {
		if err := stopPlaceErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(place)
}

// DeleteStopPlaceInput is DeleteStopPlace's request.
type DeleteStopPlaceInput struct {
	StopPlaceID string `path:"stop_place_id"`
}

// DeleteStopPlace is DELETE /stop-places/{stop_place_id}: Delete stop place.
func (ctrl *TrackingController) DeleteStopPlace(ctx context.Context, in *DeleteStopPlaceInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	stopPlaceID, err := uuid.Parse(in.StopPlaceID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.trackingService.DeleteStopPlace(c.Request.Context(), tenantID, stopPlaceID); err != nil {
		if err := stopPlaceErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== ROUTE STOPS ====================

// ListRouteStopsInput is ListRouteStops's request.
type ListRouteStopsInput struct {
	RouteID string `path:"route_id"`
	Query   api.Query[models.ListRouteStopsQuery]
}

// ListRouteStops is GET /routes/{route_id}/stops: List the stops a route runs on a date.
func (ctrl *TrackingController) ListRouteStops(ctx context.Context, in *ListRouteStopsInput) (*api.Out[[]*models.RouteStop], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.ListRouteStopsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	stops, err := ctrl.trackingService.ListRouteStops(c.Request.Context(), tenantID, routeID, query.Date, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(stops)
}

// GetRoutePathInput is GetRoutePath's request.
type GetRoutePathInput struct {
	RouteID     string `path:"route_id"`
	IfNoneMatch string `header:"If-None-Match" doc:"ETag of the line the client holds"`
	Query       api.Query[models.RoutePathQuery]
}

// GetRoutePath is the line by streets through the stops of the route version in force on `date` —
// the route's local today when unset (TRACK-028). `path` is null while no line is stored. ETag is the
// version and when its line was stored; sending it back as If-None-Match answers 304.
func (ctrl *TrackingController) GetRoutePath(ctx context.Context, in *GetRoutePathInput) (*api.Cached[models.RoutePathResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.RoutePathQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	version, err := ctrl.trackingService.GetRoutePath(c.Request.Context(), tenantID, routeID, query.Date, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	etag := version.ETag()
	response := models.RoutePathResponse{}
	if version != nil {
		response.Path = version.Path
	}
	return api.Revalidate(in.IfNoneMatch, etag, "private, max-age=0, must-revalidate", response)
}

// ==================== RIDERS CRUD ====================

// CreateRider is POST /tracking/riders in the single-database shape, which has no tenant and so no
// guardian accounts; the tenant routes serve it from AccessController.CreateRider (TRACK-037).
// CreateRiderInput is CreateRider's request.
type CreateRiderInput struct {
	JSON api.Body[models.CreateRiderDto]
}

func (ctrl *TrackingController) CreateRider(ctx context.Context, in *CreateRiderInput) (*api.Out[*models.Rider], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateRiderDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderCreateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	rider, err := ctrl.trackingService.CreateRider(c.Request.Context(), tenantID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		if err := limits.Refuse(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(rider)
}

// UpdateRiderInput is UpdateRider's request.
type UpdateRiderInput struct {
	RiderID string `path:"rider_id"`
	JSON    api.Body[models.UpdateRiderDto]
}

// UpdateRider is PATCH /riders/{rider_id}: Update rider.
func (ctrl *TrackingController) UpdateRider(ctx context.Context, in *UpdateRiderInput) (*api.Out[*models.Rider], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	riderID, err := uuid.Parse(in.RiderID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateRiderDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	conform.Strings(&dto)
	rider, err := ctrl.trackingService.UpdateRider(c.Request.Context(), tenantID, riderID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(rider)
}

// ListRidersInput is ListRiders's request.
type ListRidersInput struct {
	Query api.Query[models.ListRidersQuery]
}

// ListRiders is GET /riders: List riders.
func (ctrl *TrackingController) ListRiders(ctx context.Context, in *ListRidersInput) (*api.Out[*models.ListRidersResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListRidersQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListRiders(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetRiderInput is GetRider's request.
type GetRiderInput struct {
	RiderID string `path:"rider_id"`
}

// GetRider is GET /riders/{rider_id}: Get rider by ID.
func (ctrl *TrackingController) GetRider(ctx context.Context, in *GetRiderInput) (*api.Out[*models.Rider], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	riderID, err := uuid.Parse(in.RiderID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	rider, err := ctrl.trackingService.GetRider(c.Request.Context(), tenantID, riderID, nil, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(rider)
}

// DeleteRiderInput is DeleteRider's request.
type DeleteRiderInput struct {
	RiderID string `path:"rider_id"`
}

// DeleteRider is DELETE /riders/{rider_id}: Delete rider.
func (ctrl *TrackingController) DeleteRider(ctx context.Context, in *DeleteRiderInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	riderID, err := uuid.Parse(in.RiderID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	err = ctrl.trackingService.DeleteRider(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== ORGANIZATIONS CRUD ====================

// CreateOrganizationInput is CreateOrganization's request.
type CreateOrganizationInput struct {
	JSON api.Body[models.CreateOrganizationDto]
}

// CreateOrganization is POST /organizations: Create organization.
func (ctrl *TrackingController) CreateOrganization(ctx context.Context, in *CreateOrganizationInput) (*api.Out[*models.Organization], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateOrganizationDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	organization, err := ctrl.trackingService.CreateOrganization(c.Request.Context(), tenantID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(organization)
}

// UpdateOrganizationInput is UpdateOrganization's request.
type UpdateOrganizationInput struct {
	OrganizationID string `path:"organization_id"`
	JSON           api.Body[models.UpdateOrganizationDto]
}

// UpdateOrganization is PATCH /organizations/{organization_id}: Update organization.
func (ctrl *TrackingController) UpdateOrganization(ctx context.Context, in *UpdateOrganizationInput) (*api.Out[*models.Organization], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, err := uuid.Parse(in.OrganizationID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateOrganizationDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	organization, err := ctrl.trackingService.UpdateOrganization(c.Request.Context(), tenantID, organizationID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		if err := scopeRefused(c, err); err != nil {
			return nil, err
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(organization)
}

// ListOrganizationsInput is ListOrganizations's request.
type ListOrganizationsInput struct {
	Query api.Query[models.ListOrganizationsQuery]
}

// ListOrganizations is GET /organizations: List organizations.
func (ctrl *TrackingController) ListOrganizations(ctx context.Context, in *ListOrganizationsInput) (*api.Out[*models.ListOrganizationsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListOrganizationsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListOrganizations(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetOrganizationInput is GetOrganization's request.
type GetOrganizationInput struct {
	OrganizationID string `path:"organization_id"`
}

// GetOrganization is GET /organizations/{organization_id}: Get organization by ID.
func (ctrl *TrackingController) GetOrganization(ctx context.Context, in *GetOrganizationInput) (*api.Out[*models.Organization], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, err := uuid.Parse(in.OrganizationID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	organization, err := ctrl.trackingService.GetOrganization(c.Request.Context(), tenantID, organizationID, ctrl.scopeUserID(c))
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(organization)
}

// DeleteOrganizationInput is DeleteOrganization's request.
type DeleteOrganizationInput struct {
	OrganizationID string `path:"organization_id"`
}

// DeleteOrganization is DELETE /organizations/{organization_id}: Delete organization.
func (ctrl *TrackingController) DeleteOrganization(ctx context.Context, in *DeleteOrganizationInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, err := uuid.Parse(in.OrganizationID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	err = ctrl.trackingService.DeleteOrganization(c.Request.Context(), tenantID, organizationID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.OrganizationNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			case trackingErrors.OrganizationHasRiders:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ==================== DRIVERS CRUD ====================

// TrackingControllerCreateDriverInput is CreateDriver's request.
type TrackingControllerCreateDriverInput struct {
	JSON api.Body[models.CreateDriverDto]
}

// CreateDriver is POST /drivers: Create driver.
func (ctrl *TrackingController) CreateDriver(ctx context.Context, in *TrackingControllerCreateDriverInput) (*api.Out[*models.Driver], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateDriverDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	driver, err := ctrl.trackingService.CreateDriver(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := driverErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(driver)
}

// UpdateDriverInput is UpdateDriver's request.
type UpdateDriverInput struct {
	DriverID string `path:"driver_id"`
	JSON     api.Body[models.UpdateDriverDto]
}

// UpdateDriver is PATCH /drivers/{driver_id}: Update driver.
func (ctrl *TrackingController) UpdateDriver(ctx context.Context, in *UpdateDriverInput) (*api.Out[*models.Driver], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	driverID, err := uuid.Parse(in.DriverID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateDriverDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	driver, err := ctrl.trackingService.UpdateDriver(c.Request.Context(), tenantID, driverID, &dto)
	if err != nil {
		if err := driverErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(driver)
}

// ListDriversInput is ListDrivers's request.
type ListDriversInput struct {
	Query api.Query[models.ListDriversQuery]
}

// ListDrivers is GET /drivers: List drivers.
func (ctrl *TrackingController) ListDrivers(ctx context.Context, in *ListDriversInput) (*api.Out[*models.ListDriversResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListDriversQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListDrivers(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetDriverInput is GetDriver's request.
type GetDriverInput struct {
	DriverID string `path:"driver_id"`
}

// GetDriver is GET /drivers/{driver_id}: Get driver by ID.
func (ctrl *TrackingController) GetDriver(ctx context.Context, in *GetDriverInput) (*api.Out[*models.Driver], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	driverID, err := uuid.Parse(in.DriverID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	driver, err := ctrl.trackingService.GetDriver(c.Request.Context(), tenantID, driverID)
	if err != nil {
		if err := driverErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(driver)
}

// DeleteDriverInput is DeleteDriver's request.
type DeleteDriverInput struct {
	DriverID string `path:"driver_id"`
}

// DeleteDriver is DELETE /drivers/{driver_id}: Delete driver.
func (ctrl *TrackingController) DeleteDriver(ctx context.Context, in *DeleteDriverInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	driverID, err := uuid.Parse(in.DriverID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.trackingService.DeleteDriver(c.Request.Context(), tenantID, driverID); err != nil {
		if err := driverErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// driverErrorResponse answers the driver codes that carry a status of their own and reports whether
// it did; anything else is the caller's 500. One map keeps the five handlers from disagreeing about
// what a duplicate licence is.
// routeErrorResponse answers the route SPs' own refusals. company-mismatch carries the offending
// field (vehicle_id or default_driver_id) as its detail, so the form can put it on that field;
// service.blocked does the same (TRACK-064).
func routeErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.CompanyNotFound, trackingErrors.OrganizationNotFound,
		trackingErrors.StopPlaceNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.RouteDestinationRequired, trackingErrors.RouteDepartureRequired, trackingErrors.StopPlaceInvalid:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.RouteCompanyMismatch:
		var pgErr *pgconn.PgError
		detail := ""
		if errors.As(err, &pgErr) {
			detail = pgErr.Detail
		}
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErr.Code, detail))
	case trackingErrors.RouteTimezone:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.RouteCodeAlreadyExists, trackingErrors.RouteHasReturn, trackingErrors.RouteNotOutbound:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.ServiceBlocked:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorDetail(c, trackingErr.Code, trackingErr.Detail))
	}
	return nil
}

func driverErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.DriverNotFound, trackingErrors.CompanyNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.DriverLicenseAlreadyExists, trackingErrors.DriverUserAlreadyLinked, trackingErrors.DriverHasRoutes:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// ==================== DOCUMENT TYPES (TRACK-016) ====================

// CreateDocumentTypeInput is CreateDocumentType's request.
type CreateDocumentTypeInput struct {
	JSON api.Body[models.CreateDocumentTypeDto]
}

// CreateDocumentType is POST /document-types: Create document type.
func (ctrl *TrackingController) CreateDocumentType(ctx context.Context, in *CreateDocumentTypeInput) (*api.Out[*models.DocumentType], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateDocumentTypeDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentTypeCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	docType, err := ctrl.trackingService.CreateDocumentType(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(docType)
}

// UpdateDocumentTypeInput is UpdateDocumentType's request.
type UpdateDocumentTypeInput struct {
	DocumentTypeID string `path:"document_type_id"`
	JSON           api.Body[models.UpdateDocumentTypeDto]
}

// UpdateDocumentType is PATCH /document-types/{document_type_id}: Update document type.
func (ctrl *TrackingController) UpdateDocumentType(ctx context.Context, in *UpdateDocumentTypeInput) (*api.Out[*models.DocumentType], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	typeID, err := uuid.Parse(in.DocumentTypeID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateDocumentTypeDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentTypeUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	docType, err := ctrl.trackingService.UpdateDocumentType(c.Request.Context(), tenantID, typeID, &dto)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(docType)
}

// ListDocumentTypesInput is ListDocumentTypes's request.
type ListDocumentTypesInput struct {
	Query api.Query[models.ListDocumentTypesQuery]
}

// ListDocumentTypes is GET /document-types: List document types.
func (ctrl *TrackingController) ListDocumentTypes(ctx context.Context, in *ListDocumentTypesInput) (*api.Out[[]*models.DocumentType], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListDocumentTypesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	types, err := ctrl.trackingService.ListDocumentTypes(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(types)
}

// ==================== COMPLIANCE DOCUMENTS (TRACK-016) ====================

// CreateDocumentInput is CreateDocument's request.
type CreateDocumentInput struct {
	JSON api.Body[models.CreateDocumentDto]
}

// CreateDocument is POST /documents: Create document.
func (ctrl *TrackingController) CreateDocument(ctx context.Context, in *CreateDocumentInput) (*api.Out[*models.ComplianceDocument], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateDocumentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentCreateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	document, err := ctrl.trackingService.CreateDocument(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(document)
}

// UpdateDocumentInput is UpdateDocument's request.
type UpdateDocumentInput struct {
	DocumentID string `path:"document_id"`
	JSON       api.Body[models.UpdateDocumentDto]
}

// UpdateDocument is PATCH /documents/{document_id}: Update document.
func (ctrl *TrackingController) UpdateDocument(ctx context.Context, in *UpdateDocumentInput) (*api.Out[*models.ComplianceDocument], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateDocumentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	document, err := ctrl.trackingService.UpdateDocument(c.Request.Context(), tenantID, documentID, &dto)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(document)
}

// ListDocumentsInput is ListDocuments's request.
type ListDocumentsInput struct {
	Query api.Query[models.ListDocumentsQuery]
}

// ListDocuments is GET /documents: List documents.
func (ctrl *TrackingController) ListDocuments(ctx context.Context, in *ListDocumentsInput) (*api.Out[*models.ListDocumentsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	var query models.ListDocumentsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	result, err := ctrl.trackingService.ListDocuments(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// GetDocumentInput is GetDocument's request.
type GetDocumentInput struct {
	DocumentID string `path:"document_id"`
}

// GetDocument is GET /documents/{document_id}: Get document by ID.
func (ctrl *TrackingController) GetDocument(ctx context.Context, in *GetDocumentInput) (*api.Out[*models.ComplianceDocument], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	document, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(document)
}

// DeleteDocumentInput is DeleteDocument's request.
type DeleteDocumentInput struct {
	DocumentID string `path:"document_id"`
}

// DeleteDocument is DELETE /documents/{document_id}: Delete document.
func (ctrl *TrackingController) DeleteDocument(ctx context.Context, in *DeleteDocumentInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	fileExt, err := ctrl.trackingService.DeleteDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	// The row is gone either way; a file that cannot be removed is a leak to clean up, never a
	// reason to tell the caller the delete failed.
	ctrl.documentFiles.Remove(c.Request.Context(), tenantID, documentID, fileExt)

	return api.Done()
}

// UploadDocumentFileInput is UploadDocumentFile's request.
type UploadDocumentFileInput struct {
	DocumentID string `path:"document_id"`
	JSON       api.Body[models.ClaimDocumentFileDto]
}

// UploadDocumentFile claims a scan uploaded on a ticket from POST /storage/uploads; PDF, JPG or PNG
// within the size limit.
func (ctrl *TrackingController) UploadDocumentFile(ctx context.Context, in *UploadDocumentFileInput) (*api.Out[*models.ComplianceDocument], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	var dto models.ClaimDocumentFileDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentFileInvalid, utils.ExtractValidationError(c, err)))
	}

	// The document is read first: a claim against one that is not this tenant's must be a 404
	// before a single object is touched. Its current extension says what the claim replaces.
	previous, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	ext, size, err := ctrl.documentFiles.Claim(c.Request.Context(), tenantID, documentID, dto.Key)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	document, err := ctrl.trackingService.SetDocumentFile(c.Request.Context(), tenantID, documentID,
		trackingServices.DisplayName(dto.Filename), size, ext)
	if err != nil {
		// The row never learnt about the copy, so the copy is litter — take it back out.
		ctrl.documentFiles.Discard(c.Request.Context(), tenantID, documentID, dto.Key, previous.FileExt, ext)
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	ctrl.documentFiles.AfterClaim(c.Request.Context(), tenantID, documentID, dto.Key, previous.FileExt, ext)
	return api.OK(document)
}

// DownloadDocumentFileInput is DownloadDocumentFile's request.
type DownloadDocumentFileInput struct {
	DocumentID string `path:"document_id"`
}

// DownloadDocumentFile is GET /documents/{document_id}/file: Download a document's file.
func (ctrl *TrackingController) DownloadDocumentFile(ctx context.Context, in *DownloadDocumentFileInput) (*api.Out[models.DocumentFileLink], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	documentID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	document, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	url, expiresAt, err := ctrl.documentFiles.DownloadURL(c.Request.Context(), tenantID, documentID, document.FileExt, downloadFilename(document))
	if err != nil {
		if err := documentErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(models.DocumentFileLink{URL: url, ExpiresAt: expiresAt})
}

// downloadFilename names the file after what it is — "Insurance-POL-1.pdf" — rather than after what
// the uploader called it, which never reaches the filesystem or a header (security.md).
func downloadFilename(document *models.ComplianceDocument) string {
	parts := []string{safeFilenamePart(document.TypeName)}
	if document.Number != nil && *document.Number != "" {
		parts = append(parts, safeFilenamePart(*document.Number))
	}
	name := strings.Join(parts, "-")
	if document.FileExt != nil && *document.FileExt != "" {
		name += "." + *document.FileExt
	}
	return name
}

// safeFilenamePart keeps letters, digits and dashes and drops everything else, so no quote, slash or
// newline from a stored value can break out of the Content-Disposition header.
func safeFilenamePart(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			out.WriteRune(r)
		case r == ' ' || r == '_':
			out.WriteRune('-')
		}
	}
	if out.Len() == 0 {
		return "document"
	}
	return out.String()
}

// documentErrorResponse answers the document codes that carry a status of their own and reports
// whether it did; anything else is the caller's 500.
func documentErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.DocumentNotFound, trackingErrors.DocumentTypeNotFound,
		trackingErrors.DocumentSubjectNotFound, trackingErrors.DocumentFileNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.DocumentTypeCodeAlreadyExists, trackingErrors.DocumentTypeNotApplicable:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.DocumentFileInvalid:
		return api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// ==================== ORGANIZATION MEMBERS ====================

// ListOrganizationMembersInput is ListOrganizationMembers's request.
type ListOrganizationMembersInput struct {
	OrganizationID string `path:"organization_id"`
}

// ListOrganizationMembers is GET /organizations/{organization_id}/members: List organization members.
func (ctrl *TrackingController) ListOrganizationMembers(ctx context.Context, in *ListOrganizationMembersInput) (*api.Out[[]*models.OrganizationMember], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, err := uuid.Parse(in.OrganizationID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	members, err := ctrl.trackingService.ListOrganizationMembers(c.Request.Context(), tenantID, organizationID, ctrl.scopeUserID(c))
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(members)
}

// TrackingControllerUpsertOrganizationMemberInput is UpsertOrganizationMember's request.
type TrackingControllerUpsertOrganizationMemberInput struct {
	OrganizationID string `path:"organization_id"`
	UserID         string `path:"user_id"`
	JSON           api.Body[models.UpsertOrganizationMemberDto]
}

// UpsertOrganizationMember is PUT /organizations/{organization_id}/members/{user_id}: Add an organization member or change their role.
func (ctrl *TrackingController) UpsertOrganizationMember(ctx context.Context, in *TrackingControllerUpsertOrganizationMemberInput) (*api.Out[*models.OrganizationMember], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, userID, err := ctrl.memberPathIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.UpsertOrganizationMemberDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationMemberSaveFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)

	member, err := ctrl.trackingService.UpsertOrganizationMember(c.Request.Context(), tenantID, organizationID, userID, dto.Role, dto.Groups)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) &&
			(trackingErr.Code == trackingErrors.OrganizationNotFound || trackingErr.Code == trackingErrors.OrganizationMemberUserNotFound) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(member)
}

// DeleteOrganizationMemberInput is DeleteOrganizationMember's request.
type DeleteOrganizationMemberInput struct {
	OrganizationID string `path:"organization_id"`
	UserID         string `path:"user_id"`
}

// DeleteOrganizationMember is DELETE /organizations/{organization_id}/members/{user_id}: Remove an organization member.
func (ctrl *TrackingController) DeleteOrganizationMember(ctx context.Context, _ *DeleteOrganizationMemberInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}

	organizationID, userID, err := ctrl.memberPathIDs(c)
	if err != nil {
		return nil, err
	}

	err = ctrl.trackingService.DeleteOrganizationMember(c.Request.Context(), tenantID, organizationID, userID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) &&
			(trackingErr.Code == trackingErrors.OrganizationNotFound || trackingErr.Code == trackingErrors.OrganizationMemberNotFound) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// memberPathIDs parses the two ids every membership route carries, answering 400 on either.
func (ctrl *TrackingController) memberPathIDs(c *gin.Context) (uuid.UUID, uuid.UUID, error) {
	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	return organizationID, userID, nil
}
