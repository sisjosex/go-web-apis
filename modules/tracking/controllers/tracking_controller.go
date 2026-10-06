package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leebenson/conform"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/limits"
	"josex/web/modules/core/utils"
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
}

func NewTrackingController(trackingService interfaces.TrackingService, documentFiles *trackingServices.DocumentFiles) *TrackingController {
	return &TrackingController{
		trackingService: trackingService,
		documentFiles:   documentFiles,
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
func (ctrl *TrackingController) requireTenantID(c *gin.Context) (uuid.UUID, bool) {
	tenantID, err := ctrl.getTenantID(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return uuid.Nil, false
	}
	if tenantID == nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErrors.TenantRequired))
		return uuid.Nil, false
	}
	return *tenantID, true
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
func scopeRefused(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationScopeDenied {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// GetVehicleCurrentLocation godoc
// @Summary Get vehicle current location
// @Description Retrieve the most recent GPS location for a vehicle
// @Tags Tracking - Locations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID (UUID)"
// @Success 200 {object} models.CurrentLocationResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{id}/location [get]
func (ctrl *TrackingController) GetVehicleCurrentLocation(c *gin.Context) {
	vehicleIDStr := c.Param("vehicle_id")
	vehicleID, err := uuid.Parse(vehicleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	response, err := ctrl.trackingService.GetVehicleCurrentLocation(c.Request.Context(), vehicleID)
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.VehicleNotFound, trackingErrors.LocationNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRouteRealtimeStatus godoc
// @Summary Get route real-time status
// @Description Get comprehensive route status: vehicle location, rider counts, alerts
// @Tags Tracking - Status
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Success 200 {object} models.RouteRealtimeStatusResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/status [get]
func (ctrl *TrackingController) GetRouteRealtimeStatus(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeIDStr := c.Param("route_id")
	routeID, err := uuid.Parse(routeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	response, err := ctrl.trackingService.GetRouteRealtimeStatus(c.Request.Context(), tenantID, routeID, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RouteNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRiderStatus godoc
// @Summary Get rider status (for guardians)
// @Description Get rider status including bus location, last event, and alerts (parent view)
// @Tags Tracking - Status
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider_id path string true "Rider ID (UUID)"
// @Success 200 {object} models.RiderStatusResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{rider_id}/status [get]
func (ctrl *TrackingController) GetRiderStatus(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	riderIDStr := c.Param("rider_id")
	riderID, err := uuid.Parse(riderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	response, err := ctrl.trackingService.GetRiderStatus(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RiderNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// CreateRouteAlert godoc
// @Summary Create route alert
// @Description Create a delay, breakdown, or cancellation alert for a route
// @Tags Tracking - Alerts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param alert body models.CreateAlertDto true "Alert data"
// @Success 201 {object} models.AlertCreatedResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/alerts [post]
func (ctrl *TrackingController) CreateRouteAlert(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateAlertDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AlertCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	conform.Strings(&dto)

	response, err := ctrl.trackingService.CreateRouteAlert(c.Request.Context(), tenantID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.RouteNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			default:
				c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			}
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, response)
}

// ==================== COMPANIES CRUD ====================

// CreateCompany godoc
// @Summary Create transport company
// @Description Create a new transport company (school, corporate, transport provider)
// @Tags Tracking - Companies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param company body models.CreateCompanyDto true "Company data"
// @Success 201 {object} models.TransportCompany
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/companies [post]
func (ctrl *TrackingController) CreateCompany(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateCompanyDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CompanyCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)

	company, err := ctrl.trackingService.CreateCompany(c.Request.Context(), tenantID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, company)
}

// UpdateCompany godoc
// @Summary Update transport company
// @Description Update existing transport company details
// @Tags Tracking - Companies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Company ID (UUID)"
// @Param company body models.UpdateCompanyDto true "Updated company data"
// @Success 200 {object} models.TransportCompany
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/companies/{id} [patch]
func (ctrl *TrackingController) UpdateCompany(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	companyID, err := uuid.Parse(c.Param("company_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateCompanyDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CompanyUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)

	company, err := ctrl.trackingService.UpdateCompany(c.Request.Context(), tenantID, companyID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, company)
}

// ListCompanies godoc
// @Summary List transport companies
// @Description One page of the current tenant's companies, newest first
// @Tags Tracking - Companies
// @Produce json
// @Security BearerAuth
// @Param search query string false "Match against name or registration number"
// @Param status query string false "Lifecycle state" Enums(active, inactive, suspended)
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListCompaniesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/companies [get]
func (ctrl *TrackingController) ListCompanies(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListCompaniesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListCompanies(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetCompany godoc
// @Summary Get company by ID
// @Description Get a single transport company by ID
// @Tags Tracking - Companies
// @Produce json
// @Security BearerAuth
// @Param id path string true "Company ID (UUID)"
// @Success 200 {object} models.TransportCompany
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/companies/{id} [get]
func (ctrl *TrackingController) GetCompany(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	companyID, err := uuid.Parse(c.Param("company_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	company, err := ctrl.trackingService.GetCompany(c.Request.Context(), tenantID, companyID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.CompanyNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, company)
}

// DeleteCompany godoc
// @Summary Delete transport company
// @Description Delete a transport company (owner only)
// @Tags Tracking - Companies
// @Security BearerAuth
// @Param id path string true "Company ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/companies/{id} [delete]
func (ctrl *TrackingController) DeleteCompany(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	companyID, err := uuid.Parse(c.Param("company_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	err = ctrl.trackingService.DeleteCompany(c.Request.Context(), tenantID, companyID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.CompanyNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			case trackingErrors.CompanyHasVehicles:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== VEHICLES CRUD ====================

// CreateVehicle godoc
// @Summary Create vehicle
// @Description Create a new vehicle for a transport company
// @Tags Tracking - Vehicles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param vehicle body models.CreateVehicleDto true "Vehicle data"
// @Success 201 {object} models.Vehicle
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles [post]
func (ctrl *TrackingController) CreateVehicle(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateVehicleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	vehicle, err := ctrl.trackingService.CreateVehicle(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if limits.Respond(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, vehicle)
}

// UpdateVehicle godoc
// @Summary Update vehicle
// @Description Update existing vehicle details
// @Tags Tracking - Vehicles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID (UUID)"
// @Param vehicle body models.UpdateVehicleDto true "Updated vehicle data"
// @Success 200 {object} models.Vehicle
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{id} [patch]
func (ctrl *TrackingController) UpdateVehicle(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	vehicleID, err := uuid.Parse(c.Param("vehicle_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateVehicleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.VehicleUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	vehicle, err := ctrl.trackingService.UpdateVehicle(c.Request.Context(), tenantID, vehicleID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, vehicle)
}

// ListVehicles godoc
// @Summary List vehicles
// @Description One page of the current tenant's vehicles, newest first. Without company_id the
// @Description whole fleet is listed; every row carries its company name.
// @Tags Tracking - Vehicles
// @Produce json
// @Security BearerAuth
// @Param company_id query string false "Narrow to one company (UUID)"
// @Param vehicle_type query string false "Vehicle type" Enums(bus, van, car)
// @Param status query string false "Lifecycle state" Enums(active, inactive, maintenance)
// @Param search query string false "Match against the plate number"
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListVehiclesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles [get]
func (ctrl *TrackingController) ListVehicles(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListVehiclesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListVehicles(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetVehicle godoc
// @Summary Get vehicle by ID
// @Description Get a single vehicle by ID
// @Tags Tracking - Vehicles
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID (UUID)"
// @Success 200 {object} models.Vehicle
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{id} [get]
func (ctrl *TrackingController) GetVehicle(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	vehicleID, err := uuid.Parse(c.Param("vehicle_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	vehicle, err := ctrl.trackingService.GetVehicle(c.Request.Context(), tenantID, vehicleID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.VehicleNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, vehicle)
}

// DeleteVehicle godoc
// @Summary Delete vehicle
// @Description Delete a vehicle (owner only)
// @Tags Tracking - Vehicles
// @Security BearerAuth
// @Param id path string true "Vehicle ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/vehicles/{id} [delete]
func (ctrl *TrackingController) DeleteVehicle(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	vehicleID, err := uuid.Parse(c.Param("vehicle_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	err = ctrl.trackingService.DeleteVehicle(c.Request.Context(), tenantID, vehicleID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.VehicleNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== ROUTES CRUD ====================

// CreateRoute godoc
// @Summary Create route
// @Description Create a new transportation route
// @Tags Tracking - Routes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route body models.CreateRouteDto true "Route data"
// @Success 201 {object} models.Route
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes [post]
func (ctrl *TrackingController) CreateRoute(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateRouteDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	route, err := ctrl.trackingService.CreateRoute(c.Request.Context(), tenantID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if routeErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, route)
}

// UpdateRoute godoc
// @Summary Update route
// @Description Update existing route details
// @Tags Tracking - Routes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Route ID (UUID)"
// @Param route body models.UpdateRouteDto true "Updated route data"
// @Success 200 {object} models.Route
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{id} [patch]
func (ctrl *TrackingController) UpdateRoute(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateRouteDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	route, err := ctrl.trackingService.UpdateRoute(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if routeErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, route)
}

// ListRoutes godoc
// @Summary List routes
// @Description One page of the tenant's routes by name, with company name, plate and driver name (TRACK-002 D2)
// @Tags Tracking - Routes
// @Produce json
// @Security BearerAuth
// @Param search query string false "Name or code contains"
// @Param company_id query string false "Company ID (UUID)"
// @Param direction query string false "outbound or inbound"
// @Param is_active query bool false "Active flag"
// @Param page query int false "Page (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListRoutesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes [get]
func (ctrl *TrackingController) ListRoutes(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListRoutesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListRoutes(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetRoute godoc
// @Summary Get route by ID
// @Description Get a single route by ID
// @Tags Tracking - Routes
// @Produce json
// @Security BearerAuth
// @Param id path string true "Route ID (UUID)"
// @Success 200 {object} models.Route
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{id} [get]
func (ctrl *TrackingController) GetRoute(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	route, err := ctrl.trackingService.GetRoute(c.Request.Context(), tenantID, routeID, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, route)
}

// DeleteRoute godoc
// @Summary Delete route
// @Description Delete a route (owner only)
// @Tags Tracking - Routes
// @Security BearerAuth
// @Param id path string true "Route ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{id} [delete]
func (ctrl *TrackingController) DeleteRoute(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	err = ctrl.trackingService.DeleteRoute(c.Request.Context(), tenantID, routeID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== ROUTE VERSIONS ====================

// routeVersionErrorResponse maps the route-version codes to statuses and reports whether it has
// already written the response.
func routeVersionErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.RouteVersionNotFound, trackingErrors.StopPlaceNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.RouteVersionOverlap, trackingErrors.RouteVersionClosed:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.StopPlaceInvalid:
		c.JSON(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ListRouteVersions godoc
// @Summary List a route's stop lists over time
// @Description Every version of the route's stop list, newest first, each with how many stops it holds
// @Tags Tracking - Route Versions
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Success 200 {array} models.RouteVersion
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/versions [get]
func (ctrl *TrackingController) ListRouteVersions(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	versions, err := ctrl.trackingService.ListRouteVersions(c.Request.Context(), tenantID, routeID)
	if err != nil {
		if routeVersionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, versions)
}

// CreateRouteVersion godoc
// @Summary Publish a route's next stop list
// @Description Publishes a new version from `effective_from`, closing the one in force the day before
// @Tags Tracking - Route Versions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param version body models.CreateRouteVersionDto true "The date the list takes effect and the stops on it"
// @Success 201 {object} models.RouteVersion
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/versions [post]
func (ctrl *TrackingController) CreateRouteVersion(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.CreateRouteVersionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteVersionCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	version, err := ctrl.trackingService.CreateRouteVersion(c.Request.Context(), tenantID, routeID, &dto, ctrl.actingUserID(c))
	if err != nil {
		if routeVersionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, version)
}

// ReplaceRouteVersionStops godoc
// @Summary Replace a route version's whole stop list
// @Description One request replaces the list, so a reorder never renumbers rows one at a time; refused once the version's first day has passed
// @Tags Tracking - Route Versions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param version_id path string true "Route version ID (UUID)"
// @Param stops body models.ReplaceRouteVersionStopsDto true "The whole list, in the order it runs"
// @Success 200 {array} models.RouteStop
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/versions/{version_id}/stops [put]
func (ctrl *TrackingController) ReplaceRouteVersionStops(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.ReplaceRouteVersionStopsDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteVersionSaveStops, utils.ExtractValidationError(c, err)))
		return
	}

	stops, err := ctrl.trackingService.ReplaceRouteVersionStops(c.Request.Context(), tenantID, versionID, &dto)
	if err != nil {
		if routeVersionErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, stops)
}

// ==================== STOP PLACES ====================

// stopPlaceErrorResponse maps the stop-place codes to statuses and reports whether it has already
// written the response.
func stopPlaceErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.StopPlaceNotFound, trackingErrors.OrganizationNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.StopPlaceInUse:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// CreateStopPlace godoc
// @Summary Create stop place
// @Description Create a place routes can call at; one row shared by every route that stops there (TRACK-007 D1)
// @Tags Tracking - Stop Places
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param stop_place body models.CreateStopPlaceDto true "Stop place data"
// @Success 201 {object} models.StopPlace
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/stop-places [post]
func (ctrl *TrackingController) CreateStopPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateStopPlaceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.StopPlaceCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	place, err := ctrl.trackingService.CreateStopPlace(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if stopPlaceErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, place)
}

// UpdateStopPlace godoc
// @Summary Update stop place
// @Description Edit a stop place; every field is optional and the coordinate moves only when both halves are sent
// @Tags Tracking - Stop Places
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param stop_place_id path string true "Stop place ID (UUID)"
// @Param stop_place body models.UpdateStopPlaceDto true "Updated stop place data"
// @Success 200 {object} models.StopPlace
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/stop-places/{stop_place_id} [patch]
func (ctrl *TrackingController) UpdateStopPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	stopPlaceID, err := uuid.Parse(c.Param("stop_place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateStopPlaceDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.StopPlaceUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	place, err := ctrl.trackingService.UpdateStopPlace(c.Request.Context(), tenantID, stopPlaceID, &dto)
	if err != nil {
		if stopPlaceErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, place)
}

// ListStopPlaces godoc
// @Summary List stop places
// @Description One page of the tenant's stop places by name, or — with `near` — the ones within `radius_m` of a point, nearest first and each carrying `distance_m`
// @Tags Tracking - Stop Places
// @Produce json
// @Security BearerAuth
// @Param search query string false "Match against the name"
// @Param near query string false "A point as `latitude,longitude`; turns the list into a proximity search"
// @Param radius_m query int false "How far from `near` to look, in metres" default(1000)
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListStopPlacesResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/stop-places [get]
func (ctrl *TrackingController) ListStopPlaces(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListStopPlacesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)
	latitude, longitude, err := query.NearPoint()
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	result, err := ctrl.trackingService.ListStopPlaces(c.Request.Context(), tenantID, query, latitude, longitude)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetStopPlace godoc
// @Summary Get stop place by ID
// @Description Get a single stop place by ID
// @Tags Tracking - Stop Places
// @Produce json
// @Security BearerAuth
// @Param stop_place_id path string true "Stop place ID (UUID)"
// @Success 200 {object} models.StopPlace
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/stop-places/{stop_place_id} [get]
func (ctrl *TrackingController) GetStopPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	stopPlaceID, err := uuid.Parse(c.Param("stop_place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	place, err := ctrl.trackingService.GetStopPlace(c.Request.Context(), tenantID, stopPlaceID)
	if err != nil {
		if stopPlaceErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, place)
}

// DeleteStopPlace godoc
// @Summary Delete stop place
// @Description Delete a stop place; refused while any route version still names it
// @Tags Tracking - Stop Places
// @Security BearerAuth
// @Param stop_place_id path string true "Stop place ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/stop-places/{stop_place_id} [delete]
func (ctrl *TrackingController) DeleteStopPlace(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	stopPlaceID, err := uuid.Parse(c.Param("stop_place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteStopPlace(c.Request.Context(), tenantID, stopPlaceID); err != nil {
		if stopPlaceErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== ROUTE STOPS ====================

// ListRouteStops godoc
// @Summary List the stops a route runs on a date
// @Description The ordered stops of the route version in force on `date` — today when it is unset (TRACK-007 D1)
// @Tags Tracking - Route Stops
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param date query string false "The day whose list is wanted (YYYY-MM-DD); defaults to today"
// @Success 200 {array} models.RouteStop
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/stops [get]
func (ctrl *TrackingController) ListRouteStops(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.ListRouteStopsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	stops, err := ctrl.trackingService.ListRouteStops(c.Request.Context(), tenantID, routeID, query.Date, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, stops)
}

// GetRoutePath godoc
// @Summary The planned line of a route on a date
// @Description The line by streets through the stops of the route version in force on `date` — the route's local today when unset (TRACK-028). `path` is null while no line is stored. ETag is the version and when its line was stored; sending it back as If-None-Match answers 304
// @Tags Tracking - Route Stops
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param date query string false "The day whose version is wanted (YYYY-MM-DD); defaults to the route's today"
// @Param If-None-Match header string false "ETag of the line the client holds"
// @Success 200 {object} models.RoutePathResponse
// @Success 304
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/path [get]
func (ctrl *TrackingController) GetRoutePath(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.RoutePathQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	version, err := ctrl.trackingService.GetRoutePath(c.Request.Context(), tenantID, routeID, query.Date, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	etag := version.ETag()
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	response := models.RoutePathResponse{}
	if version != nil {
		response.Path = version.Path
	}
	c.JSON(http.StatusOK, response)
}

// ==================== RIDERS CRUD ====================

// CreateRider is POST /tracking/riders in the single-database shape, which has no tenant and so no
// guardian accounts; the tenant routes serve it from AccessController.CreateRider (TRACK-037).
func (ctrl *TrackingController) CreateRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateRiderDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	rider, err := ctrl.trackingService.CreateRider(c.Request.Context(), tenantID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) || limits.Respond(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, rider)
}

// UpdateRider godoc
// @Summary Update rider
// @Description Update existing rider details
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Rider ID (UUID)"
// @Param rider body models.UpdateRiderDto true "Updated rider data"
// @Success 200 {object} models.Rider
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{id} [patch]
func (ctrl *TrackingController) UpdateRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	riderID, err := uuid.Parse(c.Param("rider_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateRiderDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	rider, err := ctrl.trackingService.UpdateRider(c.Request.Context(), tenantID, riderID, &dto, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, rider)
}

// ListRiders godoc
// @Summary List riders
// @Description One page of the current tenant's riders, alphabetically
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param organization_id query string false "Organization ID (UUID); unset lists the whole tenant"
// @Param rider_type query string false "Rider type" Enums(student, employee)
// @Param is_active query bool false "Lifecycle state"
// @Param search query string false "Match against first name, last name or identification number"
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListRidersResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders [get]
func (ctrl *TrackingController) ListRiders(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListRidersQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListRiders(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c), ctrl.guardianUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetRider godoc
// @Summary Get rider by ID
// @Description Get a single rider by ID
// @Tags Tracking - Riders
// @Produce json
// @Security BearerAuth
// @Param id path string true "Rider ID (UUID)"
// @Success 200 {object} models.Rider
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{id} [get]
func (ctrl *TrackingController) GetRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	riderID, err := uuid.Parse(c.Param("rider_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	rider, err := ctrl.trackingService.GetRider(c.Request.Context(), tenantID, riderID, nil, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, rider)
}

// DeleteRider godoc
// @Summary Delete rider
// @Description Delete a rider (owner only)
// @Tags Tracking - Riders
// @Security BearerAuth
// @Param id path string true "Rider ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders/{id} [delete]
func (ctrl *TrackingController) DeleteRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	riderID, err := uuid.Parse(c.Param("rider_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	err = ctrl.trackingService.DeleteRider(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== ORGANIZATIONS CRUD ====================

// CreateOrganization godoc
// @Summary Create organization
// @Description Create a school or employer whose people ride
// @Tags Tracking - Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization body models.CreateOrganizationDto true "Organization data"
// @Success 201 {object} models.Organization
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations [post]
func (ctrl *TrackingController) CreateOrganization(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateOrganizationDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	organization, err := ctrl.trackingService.CreateOrganization(c.Request.Context(), tenantID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, organization)
}

// UpdateOrganization godoc
// @Summary Update organization
// @Description Update an organization; every field is optional
// @Tags Tracking - Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param organization body models.UpdateOrganizationDto true "Updated organization data"
// @Success 200 {object} models.Organization
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id} [patch]
func (ctrl *TrackingController) UpdateOrganization(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateOrganizationDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	organization, err := ctrl.trackingService.UpdateOrganization(c.Request.Context(), tenantID, organizationID, &dto)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, organization)
}

// ListOrganizations godoc
// @Summary List organizations
// @Description One page of the current tenant's organizations, alphabetically
// @Tags Tracking - Organizations
// @Produce json
// @Security BearerAuth
// @Param search query string false "Match against the name"
// @Param kind query string false "Organization kind" Enums(school, company, other)
// @Param is_active query bool false "Lifecycle state"
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListOrganizationsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations [get]
func (ctrl *TrackingController) ListOrganizations(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListOrganizationsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListOrganizations(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetOrganization godoc
// @Summary Get organization by ID
// @Description Get a single organization by ID
// @Tags Tracking - Organizations
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Success 200 {object} models.Organization
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id} [get]
func (ctrl *TrackingController) GetOrganization(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	organization, err := ctrl.trackingService.GetOrganization(c.Request.Context(), tenantID, organizationID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, organization)
}

// DeleteOrganization godoc
// @Summary Delete organization
// @Description Delete an organization; refused while it still has riders
// @Tags Tracking - Organizations
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id} [delete]
func (ctrl *TrackingController) DeleteOrganization(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	err = ctrl.trackingService.DeleteOrganization(c.Request.Context(), tenantID, organizationID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.OrganizationNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			case trackingErrors.OrganizationHasRiders:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== DRIVERS CRUD ====================

// CreateDriver godoc
// @Summary Create driver
// @Description Create a driver for one of the tenant's carriers
// @Tags Tracking - Drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param driver body models.CreateDriverDto true "Driver data"
// @Success 201 {object} models.Driver
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers [post]
func (ctrl *TrackingController) CreateDriver(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateDriverDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	driver, err := ctrl.trackingService.CreateDriver(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if driverErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, driver)
}

// UpdateDriver godoc
// @Summary Update driver
// @Description Update a driver; every field is optional and the carrier is immutable
// @Tags Tracking - Drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param driver_id path string true "Driver ID (UUID)"
// @Param driver body models.UpdateDriverDto true "Updated driver data"
// @Success 200 {object} models.Driver
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers/{driver_id} [patch]
func (ctrl *TrackingController) UpdateDriver(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	driverID, err := uuid.Parse(c.Param("driver_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateDriverDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DriverUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	driver, err := ctrl.trackingService.UpdateDriver(c.Request.Context(), tenantID, driverID, &dto)
	if err != nil {
		if driverErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, driver)
}

// ListDrivers godoc
// @Summary List drivers
// @Description One page of the current tenant's drivers, by surname
// @Tags Tracking - Drivers
// @Produce json
// @Security BearerAuth
// @Param search query string false "Match against a name or the licence number"
// @Param company_id query string false "Carrier ID (UUID); unset lists every carrier"
// @Param status query string false "Lifecycle state" Enums(active, inactive, suspended)
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListDriversResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers [get]
func (ctrl *TrackingController) ListDrivers(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListDriversQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.trackingService.ListDrivers(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetDriver godoc
// @Summary Get driver by ID
// @Description Get a single driver by ID
// @Tags Tracking - Drivers
// @Produce json
// @Security BearerAuth
// @Param driver_id path string true "Driver ID (UUID)"
// @Success 200 {object} models.Driver
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers/{driver_id} [get]
func (ctrl *TrackingController) GetDriver(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	driverID, err := uuid.Parse(c.Param("driver_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	driver, err := ctrl.trackingService.GetDriver(c.Request.Context(), tenantID, driverID)
	if err != nil {
		if driverErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, driver)
}

// DeleteDriver godoc
// @Summary Delete driver
// @Description Delete a driver; refused while a route still names them as its default driver
// @Tags Tracking - Drivers
// @Security BearerAuth
// @Param driver_id path string true "Driver ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/drivers/{driver_id} [delete]
func (ctrl *TrackingController) DeleteDriver(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	driverID, err := uuid.Parse(c.Param("driver_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteDriver(c.Request.Context(), tenantID, driverID); err != nil {
		if driverErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// driverErrorResponse answers the driver codes that carry a status of their own and reports whether
// it did; anything else is the caller's 500. One map keeps the five handlers from disagreeing about
// what a duplicate licence is.
// routeErrorResponse answers the route SPs' own refusals. company-mismatch carries the offending
// field (vehicle_id or default_driver_id) as its detail, so the form can put it on that field.
func routeErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.CompanyNotFound, trackingErrors.OrganizationNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.RouteDestinationRequired, trackingErrors.RouteDepartureRequired, trackingErrors.StopPlaceInvalid:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.RouteCompanyMismatch:
		var pgErr *pgconn.PgError
		detail := ""
		if errors.As(err, &pgErr) {
			detail = pgErr.Detail
		}
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErr.Code, detail))
		return true
	case trackingErrors.RouteTimezone:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.RouteCodeAlreadyExists:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

func driverErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.DriverNotFound, trackingErrors.CompanyNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.DriverLicenseAlreadyExists, trackingErrors.DriverUserAlreadyLinked, trackingErrors.DriverHasRoutes:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ==================== DOCUMENT TYPES (TRACK-016) ====================

// CreateDocumentType godoc
// @Summary Create document type
// @Description Add a line to the tenant's compliance policy
// @Tags Tracking - Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param document_type body models.CreateDocumentTypeDto true "Document type"
// @Success 201 {object} models.DocumentType
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/document-types [post]
func (ctrl *TrackingController) CreateDocumentType(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateDocumentTypeDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentTypeCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	docType, err := ctrl.trackingService.CreateDocumentType(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, docType)
}

// UpdateDocumentType godoc
// @Summary Update document type
// @Description Edit a policy line; every field is optional
// @Tags Tracking - Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param document_type_id path string true "Document type ID (UUID)"
// @Param document_type body models.UpdateDocumentTypeDto true "Updated document type"
// @Success 200 {object} models.DocumentType
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/document-types/{document_type_id} [patch]
func (ctrl *TrackingController) UpdateDocumentType(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	typeID, err := uuid.Parse(c.Param("document_type_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateDocumentTypeDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentTypeUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	docType, err := ctrl.trackingService.UpdateDocumentType(c.Request.Context(), tenantID, typeID, &dto)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, docType)
}

// ListDocumentTypes godoc
// @Summary List document types
// @Description The tenant's whole compliance policy, alphabetically
// @Tags Tracking - Documents
// @Produce json
// @Security BearerAuth
// @Param applies_to query string false "Narrow to the types one subject kind may carry" Enums(vehicle, driver, both)
// @Param is_active query bool false "Lifecycle state"
// @Success 200 {array} models.DocumentType
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/document-types [get]
func (ctrl *TrackingController) ListDocumentTypes(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListDocumentTypesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	types, err := ctrl.trackingService.ListDocumentTypes(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, types)
}

// ==================== COMPLIANCE DOCUMENTS (TRACK-016) ====================

// CreateDocument godoc
// @Summary Create document
// @Description File a compliance document against one vehicle or driver
// @Tags Tracking - Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param document body models.CreateDocumentDto true "Document"
// @Success 201 {object} models.ComplianceDocument
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents [post]
func (ctrl *TrackingController) CreateDocument(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateDocumentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	document, err := ctrl.trackingService.CreateDocument(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, document)
}

// UpdateDocument godoc
// @Summary Update document
// @Description Edit a document; the subject and the type are set once
// @Tags Tracking - Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param document_id path string true "Document ID (UUID)"
// @Param document body models.UpdateDocumentDto true "Updated document"
// @Success 200 {object} models.ComplianceDocument
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents/{document_id} [patch]
func (ctrl *TrackingController) UpdateDocument(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	documentID, err := uuid.Parse(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateDocumentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	document, err := ctrl.trackingService.UpdateDocument(c.Request.Context(), tenantID, documentID, &dto)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, document)
}

// ListDocuments godoc
// @Summary List documents
// @Description One page of the tenant's compliance documents, soonest expiry first
// @Tags Tracking - Documents
// @Produce json
// @Security BearerAuth
// @Param subject_type query string false "Narrow to vehicles or drivers" Enums(vehicle, driver)
// @Param subject_id query string false "Narrow to one vehicle or driver (UUID)"
// @Param expiring_within_days query int false "Only what expires within N days"
// @Param page query int false "Page number, 1-based" default(1)
// @Param page_size query int false "Rows per page, max 100" default(20)
// @Success 200 {object} models.ListDocumentsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents [get]
func (ctrl *TrackingController) ListDocuments(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var query models.ListDocumentsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	result, err := ctrl.trackingService.ListDocuments(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetDocument godoc
// @Summary Get document by ID
// @Description Get a single compliance document by ID
// @Tags Tracking - Documents
// @Produce json
// @Security BearerAuth
// @Param document_id path string true "Document ID (UUID)"
// @Success 200 {object} models.ComplianceDocument
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents/{document_id} [get]
func (ctrl *TrackingController) GetDocument(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	documentID, err := uuid.Parse(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	document, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, document)
}

// DeleteDocument godoc
// @Summary Delete document
// @Description Withdraw a compliance document; its stored file goes with it
// @Tags Tracking - Documents
// @Security BearerAuth
// @Param document_id path string true "Document ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents/{document_id} [delete]
func (ctrl *TrackingController) DeleteDocument(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	documentID, err := uuid.Parse(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	fileExt, err := ctrl.trackingService.DeleteDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	// The row is gone either way; a file that cannot be removed is a leak to clean up, never a
	// reason to tell the caller the delete failed.
	ctrl.documentFiles.Remove(c.Request.Context(), tenantID, documentID, fileExt)

	c.Status(http.StatusNoContent)
}

// UploadDocumentFile godoc
// @Summary Attach a document's file
// @Description Claim a scan uploaded on a ticket from POST /storage/uploads; PDF, JPG or PNG within the size limit
// @Tags Tracking - Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param document_id path string true "Document ID (UUID)"
// @Param file body models.ClaimDocumentFileDto true "The uploaded key and the file's own name"
// @Success 200 {object} models.ComplianceDocument
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 422 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents/{document_id}/file [put]
func (ctrl *TrackingController) UploadDocumentFile(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	documentID, err := uuid.Parse(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	var dto models.ClaimDocumentFileDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.DocumentFileInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	// The document is read first: a claim against one that is not this tenant's must be a 404
	// before a single object is touched. Its current extension says what the claim replaces.
	previous, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	ext, size, err := ctrl.documentFiles.Claim(c.Request.Context(), tenantID, documentID, dto.Key)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	document, err := ctrl.trackingService.SetDocumentFile(c.Request.Context(), tenantID, documentID,
		trackingServices.DisplayName(dto.Filename), size, ext)
	if err != nil {
		// The row never learnt about the copy, so the copy is litter — take it back out.
		ctrl.documentFiles.Discard(c.Request.Context(), tenantID, documentID, dto.Key, previous.FileExt, ext)
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, document)
	ctrl.documentFiles.AfterClaim(c.Request.Context(), tenantID, documentID, dto.Key, previous.FileExt, ext)
}

// DownloadDocumentFile godoc
// @Summary Download a document's file
// @Description A 60-second signed link to the stored scan, issued behind auth; the scan is never public
// @Tags Tracking - Documents
// @Produce json
// @Security BearerAuth
// @Param document_id path string true "Document ID (UUID)"
// @Success 200 {object} models.DocumentFileLink
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/documents/{document_id}/file [get]
func (ctrl *TrackingController) DownloadDocumentFile(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	documentID, err := uuid.Parse(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	document, err := ctrl.trackingService.GetDocument(c.Request.Context(), tenantID, documentID)
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	url, expiresAt, err := ctrl.documentFiles.DownloadURL(c.Request.Context(), tenantID, documentID, document.FileExt, downloadFilename(document))
	if err != nil {
		if documentErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, models.DocumentFileLink{URL: url, ExpiresAt: expiresAt})
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
func documentErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.DocumentNotFound, trackingErrors.DocumentTypeNotFound,
		trackingErrors.DocumentSubjectNotFound, trackingErrors.DocumentFileNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.DocumentTypeCodeAlreadyExists, trackingErrors.DocumentTypeNotApplicable:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.DocumentFileInvalid:
		c.JSON(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ==================== ORGANIZATION MEMBERS ====================

// ListOrganizationMembers godoc
// @Summary List organization members
// @Description The tenant users on an organization, with their identity and role
// @Tags Tracking - Organizations
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Success 200 {array} models.OrganizationMember
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members [get]
func (ctrl *TrackingController) ListOrganizationMembers(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	members, err := ctrl.trackingService.ListOrganizationMembers(c.Request.Context(), tenantID, organizationID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.OrganizationNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, members)
}

// UpsertOrganizationMember godoc
// @Summary Add an organization member or change their role
// @Description Idempotent: the same call adds a member and re-roles one already there
// @Tags Tracking - Organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param user_id path string true "User ID (UUID)"
// @Param member body models.UpsertOrganizationMemberDto true "Member role"
// @Success 200 {object} models.OrganizationMember
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members/{user_id} [put]
func (ctrl *TrackingController) UpsertOrganizationMember(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, userID, ok := ctrl.memberPathIDs(c)
	if !ok {
		return
	}
	var dto models.UpsertOrganizationMemberDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.OrganizationMemberSaveFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)

	member, err := ctrl.trackingService.UpsertOrganizationMember(c.Request.Context(), tenantID, organizationID, userID, dto.Role)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) &&
			(trackingErr.Code == trackingErrors.OrganizationNotFound || trackingErr.Code == trackingErrors.OrganizationMemberUserNotFound) {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, member)
}

// DeleteOrganizationMember godoc
// @Summary Remove an organization member
// @Description Take a user off an organization; the user account itself is untouched
// @Tags Tracking - Organizations
// @Security BearerAuth
// @Param organization_id path string true "Organization ID (UUID)"
// @Param user_id path string true "User ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/organizations/{organization_id}/members/{user_id} [delete]
func (ctrl *TrackingController) DeleteOrganizationMember(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	organizationID, userID, ok := ctrl.memberPathIDs(c)
	if !ok {
		return
	}

	err := ctrl.trackingService.DeleteOrganizationMember(c.Request.Context(), tenantID, organizationID, userID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) &&
			(trackingErr.Code == trackingErrors.OrganizationNotFound || trackingErr.Code == trackingErrors.OrganizationMemberNotFound) {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// memberPathIDs parses the two ids every membership route carries, answering 400 on either.
func (ctrl *TrackingController) memberPathIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return uuid.Nil, uuid.Nil, false
	}
	return organizationID, userID, true
}
