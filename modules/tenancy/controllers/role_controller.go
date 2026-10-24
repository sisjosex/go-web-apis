package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
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

// ListRoles is GET /roles: List roles for a tenant.
func (ctrl *RoleController) ListRoles(ctx context.Context, _ *api.None) (*api.Out[[]*models.RoleListItem], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	roles, err := ctrl.roleService.ListRoles(c.Request.Context(), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(roles)
}

// CreateRoleInput is CreateRole's request.
type CreateRoleInput struct {
	JSON api.Body[models.CreateRoleDto]
}

// CreateRole is POST /roles: Create a role.
func (ctrl *RoleController) CreateRole(ctx context.Context, in *CreateRoleInput) (*api.Out[*models.Role], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	var dto models.CreateRoleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
	}
	dto.TenantID = tenantID

	role, err := ctrl.roleService.CreateRole(c.Request.Context(), tenantID, requesterID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.create.duplicate-name":
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDuplicateName))
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(role)
}

// GetRoleInput is GetRole's request.
type GetRoleInput struct {
	RoleID string `path:"role_id"`
}

// GetRole is GET /roles/{role_id}: Get a role.
func (ctrl *RoleController) GetRole(ctx context.Context, in *GetRoleInput) (*api.Out[*models.RoleDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	role, err := ctrl.roleService.GetRole(c.Request.Context(), tenantID, roleID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(role)
}

// UpdateRoleInput is UpdateRole's request.
type UpdateRoleInput struct {
	RoleID string `path:"role_id"`
	JSON   api.Body[models.UpdateRoleDto]
}

// UpdateRole is PATCH /roles/{role_id}: Update a role.
func (ctrl *RoleController) UpdateRole(ctx context.Context, in *UpdateRoleInput) (*api.Out[*models.Role], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	var dto models.UpdateRoleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
	}
	dto.TenantID = tenantID

	role, err := ctrl.roleService.UpdateRole(c.Request.Context(), tenantID, requesterID, roleID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// sp_update_role raises no duplicate-name of its own: the unique (tenant_id, name) index does.
			if pgErr.Code == "23505" { // unique_violation
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDuplicateName))
			}
			switch pgErr.Message {
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			case "tenant.role.update.system-role":
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleUpdateSystemRole))
			case "tenant.role.update.duplicate-name":
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDuplicateName))
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(role)
}

// DeleteRoleInput is DeleteRole's request.
type DeleteRoleInput struct {
	RoleID string `path:"role_id"`
}

// DeleteRole is DELETE /roles/{role_id}: Delete a role.
func (ctrl *RoleController) DeleteRole(ctx context.Context, in *DeleteRoleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.roleService.DeleteRole(c.Request.Context(), tenantID, requesterID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			case "tenant.role.delete.system-role":
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleDeleteSystemRole))
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.Done()
}

// SetRolePermissionsInput is SetRolePermissions's request.
type SetRolePermissionsInput struct {
	RoleID string `path:"role_id"`
	JSON   api.Body[models.SetRolePermissionsDto]
}

// SetRolePermissions is PUT /roles/{role_id}/permissions: Set role permissions.
func (ctrl *RoleController) SetRolePermissions(ctx context.Context, in *SetRolePermissionsInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	var dto models.SetRolePermissionsDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.RoleValidationFailed, err.Error()))
	}
	dto.TenantID = tenantID

	if err := ctrl.roleService.SetRolePermissions(c.Request.Context(), tenantID, requesterID, roleID, dto.Permissions); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			case "tenant.role.update.system-role":
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleUpdateSystemRole))
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.Done()
}

// AssignUserRoleInput is AssignUserRole's request.
type AssignUserRoleInput struct {
	RoleID string `path:"role_id"`
	UserID string `path:"user_id"`
}

// AssignUserRole is POST /users/{user_id}/roles/{role_id}: Assign a role to a user.
func (ctrl *RoleController) AssignUserRole(ctx context.Context, in *AssignUserRoleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	targetUserID, err := uuid.Parse(in.UserID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.roleService.AssignUserRole(c.Request.Context(), tenantID, requesterID, targetUserID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.user.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserNotFound))
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.Done()
}

// RevokeUserRoleInput is RevokeUserRole's request.
type RevokeUserRoleInput struct {
	RoleID string `path:"role_id"`
	UserID string `path:"user_id"`
}

// RevokeUserRole is DELETE /users/{user_id}/roles/{role_id}: Revoke a role from a user.
func (ctrl *RoleController) RevokeUserRole(ctx context.Context, in *RevokeUserRoleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	requesterID, err := parseRequesterID(c)
	if err != nil {
		return nil, err
	}

	targetUserID, err := uuid.Parse(in.UserID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.roleService.RevokeUserRole(c.Request.Context(), tenantID, requesterID, targetUserID, roleID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.user.unauthorized":
				return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.Done()
}

// ListUserRolesInput is ListUserRoles's request.
type ListUserRolesInput struct {
	UserID string `path:"user_id"`
}

// ListUserRoles is GET /users/{user_id}/roles: List roles assigned to a user.
func (ctrl *RoleController) ListUserRoles(ctx context.Context, in *ListUserRolesInput) (*api.Out[[]*models.UserRole], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	targetUserID, err := uuid.Parse(in.UserID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	roles, err := ctrl.roleService.ListUserRoles(c.Request.Context(), targetUserID, tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(roles)
}

// ListRoleUsersInput is ListRoleUsers's request.
type ListRoleUsersInput struct {
	RoleID string `path:"role_id"`
}

// ListRoleUsers is GET /roles/{role_id}/users: List users assigned to a role.
func (ctrl *RoleController) ListRoleUsers(ctx context.Context, in *ListRoleUsersInput) (*api.Out[[]*models.RoleUser], error) {
	c := api.Gin(ctx)
	tenantID, err := parseTenantID(c)
	if err != nil {
		return nil, err
	}

	roleID, err := uuid.Parse(in.RoleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	members, err := ctrl.roleService.ListRoleUsers(c.Request.Context(), tenantID, roleID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "tenant.role.not-found":
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, tenancyErrors.RoleNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(members)
}

// parseTenantID extracts and parses tenant_id from the Gin context (set by tenancy middleware).
// Writes an error response and returns false on failure.
func parseTenantID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get("tenant_id")
	if !exists {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}
	id, err := uuid.Parse(val.(string))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}
	return id, nil
}

// parseRequesterID extracts and parses user_id from the Gin context (set by auth middleware).
// Writes an error response and returns false on failure.
func parseRequesterID(c *gin.Context) (uuid.UUID, error) {
	val, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
	}
	id, err := uuid.Parse(val.(string))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}
	return id, nil
}
