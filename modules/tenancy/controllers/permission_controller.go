package controllers

import (
	"errors"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	coreUtils "josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// PermissionController handles HTTP requests for permission management
type PermissionController struct {
	service interfaces.PermissionService
}

// NewPermissionController creates a new PermissionController
func NewPermissionController(service interfaces.PermissionService) *PermissionController {
	return &PermissionController{service: service}
}

// ListAvailablePermissions returns all permissions that can be assigned,
// optionally filtered by ?module=<module_name>. Names and descriptions are
// translated to the request language.
func (ctrl *PermissionController) ListAvailablePermissions(c *gin.Context) {
	module := c.Query("module")
	lang := c.GetString("lang")
	perms := ctrl.service.ListAvailablePermissions(module)

	translated := make([]models.AvailablePermission, len(perms))
	for i, p := range perms {
		translated[i] = models.AvailablePermission{
			Code:        p.Code,
			Module:      p.Module,
			Name:        coreUtils.GetTranslation(lang, p.Name),
			Description: coreUtils.GetTranslation(lang, p.Description),
		}
	}
	c.JSON(http.StatusOK, translated)
}

// ListUserPermissions returns all permissions assigned to a specific user in the tenant
func (ctrl *PermissionController) ListUserPermissions(c *gin.Context) {
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	perms, err := ctrl.service.ListUserPermissions(c.Request.Context(), targetUserID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, ctrl.translateUserPermissions(c, perms))
}

// GetMyPermissions returns the current user's permissions in the tenant
func (ctrl *PermissionController) GetMyPermissions(c *gin.Context) {
	tenantIDStr, exists := c.Get("tenant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantUserUnauthorized))
		return
	}
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	userIDStr, _ := c.Get("user_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	perms, err := ctrl.service.ListUserPermissions(c.Request.Context(), userID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, ctrl.translateUserPermissions(c, perms))
}

// AssignPermission grants a permission to a user in the tenant
func (ctrl *PermissionController) AssignPermission(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	requesterIDStr, _ := c.Get("user_id")
	requesterID, err := uuid.Parse(requesterIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	var dto models.AssignPermissionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	if err := ctrl.service.AssignPermission(c.Request.Context(), tenantID, requesterID, targetUserID, dto.PermissionCode); err != nil {
		c.JSON(permissionErrorStatus(err), coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// RevokePermission removes a permission from a user in the tenant
func (ctrl *PermissionController) RevokePermission(c *gin.Context) {
	tenantIDStr, _ := c.Get("tenant_id")
	tenantID, err := uuid.Parse(tenantIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	requesterIDStr, _ := c.Get("user_id")
	requesterID, err := uuid.Parse(requesterIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	targetUserID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	var dto models.RevokePermissionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	if err := ctrl.service.RevokePermission(c.Request.Context(), tenantID, requesterID, targetUserID, dto.PermissionCode); err != nil {
		c.JSON(permissionErrorStatus(err), coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// translateUserPermissions translates Name in each UserPermissionResponse using
// the request language. The Name field from the registry is a translation key.
func (ctrl *PermissionController) translateUserPermissions(c *gin.Context, perms []*models.UserPermissionResponse) []*models.UserPermissionResponse {
	lang := c.GetString("lang")
	for _, p := range perms {
		p.Name = coreUtils.GetTranslation(lang, p.Name)
	}
	return perms
}

func permissionErrorStatus(err error) int {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "tenant.user.not-found":
			return http.StatusNotFound
		case "tenant.user.insufficient-permissions":
			return http.StatusForbidden
		case "tenant.permission.code-required":
			return http.StatusBadRequest
		}
	}
	return http.StatusInternalServerError
}
