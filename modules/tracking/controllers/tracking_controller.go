package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leebenson/conform"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"
)

type TrackingController struct {
	trackingService interfaces.TrackingService
}

func NewTrackingController(trackingService interfaces.TrackingService) *TrackingController {
	return &TrackingController{
		trackingService: trackingService,
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
	role, exists := c.Get("tenant_user_role")
	if !exists || role != tenancyModels.RoleOrganization {
		return nil
	}

	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil
	}
	return &userID
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

// UpdateVehicleLocation godoc
// @Summary Update vehicle GPS location
// @Description Insert GPS coordinates for a vehicle (high-frequency operation)
// @Tags Tracking - Locations
// @Accept json
// @Produce json
// @Param location body models.UpdateLocationDto true "GPS location data"
// @Success 200 {object} models.CurrentLocationResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/locations [post]
func (ctrl *TrackingController) UpdateVehicleLocation(c *gin.Context) {
	var dto models.UpdateLocationDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.LocationInvalidCoords, utils.ExtractValidationError(c, err)))
		return
	}

	conform.Strings(&dto)

	response, err := ctrl.trackingService.UpdateVehicleLocation(c.Request.Context(), &dto)
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.VehicleNotFound:
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

// RecordRideEvent godoc
// @Summary Record ride event (check-in/checkout)
// @Description Record when a student/employee boards, arrives, or doesn't show
// @Tags Tracking - Events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param event body models.RecordEventDto true "Ride event data"
// @Success 201 {object} models.EventRecordedResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/events [post]
func (ctrl *TrackingController) RecordRideEvent(c *gin.Context) {
	var dto models.RecordEventDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.EventInvalidType, utils.ExtractValidationError(c, err)))
		return
	}

	conform.Strings(&dto)

	userID, exists := c.Get("user_id")
	var createdBy *uuid.UUID
	if exists {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	response, err := ctrl.trackingService.RecordRideEvent(c.Request.Context(), &dto, createdBy)
	if err != nil {
		if trackingErr, ok := err.(*trackingErrors.TrackingError); ok {
			switch trackingErr.Code {
			case trackingErrors.VehicleNotFound, trackingErrors.RiderNotFound, trackingErrors.RouteNotFound:
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

	response, err := ctrl.trackingService.GetRiderStatus(c.Request.Context(), tenantID, riderID, ctrl.scopeUserID(c))
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

	userID, exists := c.Get("user_id")
	var createdBy *uuid.UUID
	if exists {
		if uid, ok := userID.(uuid.UUID); ok {
			createdBy = &uid
		}
	}

	response, err := ctrl.trackingService.CreateRouteAlert(c.Request.Context(), tenantID, &dto, createdBy)
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
	route, err := ctrl.trackingService.CreateRoute(c.Request.Context(), tenantID, &dto)
	if err != nil {
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

// ListRoutes godoc
// @Summary List routes
// @Description Get all routes for a company (scoped to current tenant)
// @Tags Tracking - Routes
// @Produce json
// @Security BearerAuth
// @Param company_id query string true "Company ID (UUID)"
// @Success 200 {array} models.Route
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes [get]
func (ctrl *TrackingController) ListRoutes(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	companyID, err := uuid.Parse(c.Query("company_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	routes, err := ctrl.trackingService.ListRoutes(c.Request.Context(), tenantID, &companyID, nil, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, routes)
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

// ==================== ROUTE STOPS CRUD ====================

// CreateRouteStop godoc
// @Summary Create route stop
// @Description Add a new stop to a route
// @Tags Tracking - Route Stops
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param stop body models.CreateRouteStopDto true "Route stop data"
// @Success 201 {object} models.RouteStop
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/route-stops [post]
func (ctrl *TrackingController) CreateRouteStop(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.CreateRouteStopDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RouteStopCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	conform.Strings(&dto)
	stop, err := ctrl.trackingService.CreateRouteStop(c.Request.Context(), tenantID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, stop)
}

// ListRouteStops godoc
// @Summary List route stops
// @Description Get all stops for a route in order
// @Tags Tracking - Route Stops
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Success 200 {array} models.RouteStop
// @Failure 400 {object} coreErrors.ErrorResponse
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
	stops, err := ctrl.trackingService.ListRouteStops(c.Request.Context(), tenantID, routeID, ctrl.scopeUserID(c))
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

// DeleteRouteStop godoc
// @Summary Delete route stop
// @Description Remove a stop from a route
// @Tags Tracking - Route Stops
// @Security BearerAuth
// @Param id path string true "Route Stop ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/route-stops/{id} [delete]
func (ctrl *TrackingController) DeleteRouteStop(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	stopID, err := uuid.Parse(c.Param("stop_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	err = ctrl.trackingService.DeleteRouteStop(c.Request.Context(), tenantID, stopID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RouteStopNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ==================== RIDERS CRUD ====================

// CreateRider godoc
// @Summary Create rider
// @Description Create a new rider (student/employee)
// @Tags Tracking - Riders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param rider body models.CreateRiderDto true "Rider data"
// @Success 201 {object} models.Rider
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/riders [post]
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
		if scopeRefused(c, err) {
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

	result, err := ctrl.trackingService.ListRiders(c.Request.Context(), tenantID, query, ctrl.scopeUserID(c))
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

// ==================== RIDER ASSIGNMENTS ====================

// AssignRider godoc
// @Summary Assign rider to route
// @Description Assign a rider to a route with pickup/dropoff stops
// @Tags Tracking - Assignments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param assignment body models.AssignRiderDto true "Assignment data"
// @Success 201 {object} models.RiderAssignment
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments [post]
func (ctrl *TrackingController) AssignRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var dto models.AssignRiderDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.AssignmentCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}
	assignment, err := ctrl.trackingService.AssignRider(c.Request.Context(), tenantID, &dto)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) {
			switch trackingErr.Code {
			case trackingErrors.RiderNotFound, trackingErrors.RouteNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			case trackingErrors.AssignmentAlreadyExists:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, assignment)
}

// UnassignRider godoc
// @Summary Unassign rider from route
// @Description Remove a rider from a route
// @Tags Tracking - Assignments
// @Security BearerAuth
// @Param id path string true "Assignment ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments/{id} [delete]
func (ctrl *TrackingController) UnassignRider(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	assignmentID, err := uuid.Parse(c.Param("assignment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	err = ctrl.trackingService.UnassignRider(c.Request.Context(), tenantID, assignmentID)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.AssignmentNotFound {
			c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ListRiderAssignments godoc
// @Summary List rider assignments
// @Description Get all route assignments (filter by rider_id, route_id, or is_active)
// @Tags Tracking - Assignments
// @Produce json
// @Security BearerAuth
// @Param rider_id query string false "Rider ID (UUID)"
// @Param route_id query string false "Route ID (UUID)"
// @Param is_active query boolean false "Filter by active status"
// @Success 200 {array} models.RiderAssignment
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/assignments [get]
func (ctrl *TrackingController) ListRiderAssignments(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}

	var riderID *uuid.UUID
	var routeID *uuid.UUID
	var isActive *bool

	if riderIDStr := c.Query("rider_id"); riderIDStr != "" {
		parsed, err := uuid.Parse(riderIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
			return
		}
		riderID = &parsed
	}

	if routeIDStr := c.Query("route_id"); routeIDStr != "" {
		parsed, err := uuid.Parse(routeIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
			return
		}
		routeID = &parsed
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		active := isActiveStr == "true"
		isActive = &active
	}

	assignments, err := ctrl.trackingService.ListRiderAssignments(c.Request.Context(), tenantID, riderID, routeID, isActive, ctrl.scopeUserID(c))
	if err != nil {
		if scopeRefused(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, assignments)
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
