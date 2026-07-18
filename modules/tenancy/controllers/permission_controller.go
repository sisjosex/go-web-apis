package controllers

import (
	"net/http"

	coreUtils "josex/web/modules/core/utils"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

// PermissionController handles HTTP requests for the permission catalog.
type PermissionController struct {
	service interfaces.PermissionService
}

// NewPermissionController creates a new PermissionController
func NewPermissionController(service interfaces.PermissionService) *PermissionController {
	return &PermissionController{service: service}
}

// ListAvailablePermissions returns all permissions that can be assigned to a
// role, optionally filtered by ?module=<module_name>. Names and descriptions are
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
