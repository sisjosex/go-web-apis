package controllers

import (
	"errors"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// RoleController handles HTTP requests for tenant role management.
type RoleController struct {
	roleService interfaces.RoleService
}

// NewRoleController creates a new RoleController.
func NewRoleController(roleService interfaces.RoleService) *RoleController {
	return &RoleController{roleService: roleService}
}

// ListRoles godoc
// @Summary      List roles for a tenant
// @Description  Returns all custom roles defined for the current tenant, including permission count
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Success      200  {array}   models.RoleListItem
// @Failure      401  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles [get]
// @Security     ApiKeyAuth
func (ctrl *RoleController) ListRoles(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	roles, err := ctrl.roleService.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, roles)
}

// CreateRole godoc
// @Summary      Create a role
// @Description  Creates a new custom role for the current tenant
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                  true  "Bearer Token"
// @Param        tenant_slug    path    string                  true  "Tenant slug"
// @Param        body           body    models.CreateRoleDto    true  "Role data"
// @Success      201  {object}  models.Role
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Failure      409  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles [post]
// @Security     ApiKeyAuth
func (ctrl *RoleController) CreateRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	var dto models.CreateRoleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
		return
	}
	dto.TenantID = tenantID

	role, err := ctrl.roleService.CreateRole(c.Request.Context(), tenantID, requesterID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.create.duplicate-name":
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDuplicateName))
				return
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, role)
}

// GetRole godoc
// @Summary      Get a role
// @Description  Returns a single role with its full permission code list
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Param        role_id        path    string  true  "Role UUID"
// @Success      200  {object}  models.RoleDetail
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      404  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles/{role_id} [get]
// @Security     ApiKeyAuth
func (ctrl *RoleController) GetRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	role, err := ctrl.roleService.GetRole(c.Request.Context(), tenantID, roleID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, role)
}

// UpdateRole godoc
// @Summary      Update a role
// @Description  Updates the name and/or description of a custom tenant role
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                  true  "Bearer Token"
// @Param        tenant_slug    path    string                  true  "Tenant slug"
// @Param        role_id        path    string                  true  "Role UUID"
// @Param        body           body    models.UpdateRoleDto    true  "Fields to update"
// @Success      200  {object}  models.Role
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Failure      404  {object}  errors.ErrorResponse
// @Failure      409  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles/{role_id} [patch]
// @Security     ApiKeyAuth
func (ctrl *RoleController) UpdateRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	var dto models.UpdateRoleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
		return
	}
	dto.TenantID = tenantID

	role, err := ctrl.roleService.UpdateRole(c.Request.Context(), tenantID, requesterID, roleID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
				return
			case "tenant.role.update.system-role":
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleUpdateSystemRole))
				return
			case "tenant.role.update.duplicate-name":
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDuplicateName))
				return
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, role)
}

// DeleteRole godoc
// @Summary      Delete a role
// @Description  Deletes a custom tenant role; system roles cannot be deleted
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Param        role_id        path    string  true  "Role UUID"
// @Success      204
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Failure      404  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles/{role_id} [delete]
// @Security     ApiKeyAuth
func (ctrl *RoleController) DeleteRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.roleService.DeleteRole(c.Request.Context(), tenantID, requesterID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
				return
			case "tenant.role.delete.system-role":
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDeleteSystemRole))
				return
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.Status(http.StatusNoContent)
}

// SetRolePermissions godoc
// @Summary      Set role permissions
// @Description  Replaces all permissions for a role atomically; pass an empty array to clear all permissions
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                       true  "Bearer Token"
// @Param        tenant_slug    path    string                       true  "Tenant slug"
// @Param        role_id        path    string                       true  "Role UUID"
// @Param        body           body    models.SetRolePermissionsDto true  "Permission codes"
// @Success      204
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Failure      404  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/roles/{role_id}/permissions [put]
// @Security     ApiKeyAuth
func (ctrl *RoleController) SetRolePermissions(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	var dto models.SetRolePermissionsDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
		return
	}
	dto.TenantID = tenantID

	if err := ctrl.roleService.SetRolePermissions(c.Request.Context(), tenantID, requesterID, roleID, dto.Permissions); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
				return
			case "tenant.role.update.system-role":
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleUpdateSystemRole))
				return
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.Status(http.StatusNoContent)
}

// AssignUserRole godoc
// @Summary      Assign a role to a user
// @Description  Assigns a custom role to a tenant user; idempotent if already assigned
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Param        user_id        path    string  true  "Target user UUID"
// @Param        role_id        path    string  true  "Role UUID"
// @Success      204
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Failure      404  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/users/{user_id}/roles/{role_id} [post]
// @Security     ApiKeyAuth
func (ctrl *RoleController) AssignUserRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.roleService.AssignUserRole(c.Request.Context(), tenantID, requesterID, targetUserID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.user.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserNotFound))
				return
			case "tenant.role.not-found":
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
				return
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.Status(http.StatusNoContent)
}

// RevokeUserRole godoc
// @Summary      Revoke a role from a user
// @Description  Removes a custom role from a tenant user; idempotent if not assigned
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Param        user_id        path    string  true  "Target user UUID"
// @Param        role_id        path    string  true  "Role UUID"
// @Success      204
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Failure      403  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/users/{user_id}/roles/{role_id} [delete]
// @Security     ApiKeyAuth
func (ctrl *RoleController) RevokeUserRole(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	requesterID, ok := parseRequesterID(c)
	if !ok {
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.roleService.RevokeUserRole(c.Request.Context(), tenantID, requesterID, targetUserID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.user.unauthorized":
				c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.Status(http.StatusNoContent)
}

// ListUserRoles godoc
// @Summary      List roles assigned to a user
// @Description  Returns all custom roles currently assigned to a user within the tenant
// @Tags         Roles
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        tenant_slug    path    string  true  "Tenant slug"
// @Param        user_id        path    string  true  "Target user UUID"
// @Success      200  {array}   models.UserRole
// @Failure      400  {object}  errors.ErrorResponse
// @Failure      401  {object}  errors.ErrorResponse
// @Router       /tenants/{tenant_slug}/users/{user_id}/roles [get]
// @Security     ApiKeyAuth
func (ctrl *RoleController) ListUserRoles(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	roles, err := ctrl.roleService.ListUserRoles(c.Request.Context(), targetUserID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, roles)
}

// parseTenantID extracts and parses tenant_id from the Gin context (set by tenancy middleware).
// Writes an error response and returns false on failure.
func parseTenantID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(val.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return uuid.Nil, false
	}
	return id, true
}

// parseRequesterID extracts and parses user_id from the Gin context (set by auth middleware).
// Writes an error response and returns false on failure.
func parseRequesterID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(val.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return uuid.Nil, false
	}
	return id, true
}
