package controllers

import (
	"context"

	"josex/web/modules/core/api"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"
)

// PermissionController handles HTTP requests for the permission catalog.
type PermissionController struct {
	service interfaces.PermissionService
}

// NewPermissionController creates a new PermissionController
func NewPermissionController(service interfaces.PermissionService) *PermissionController {
	return &PermissionController{service: service}
}

// ListAvailablePermissionsInput is ListAvailablePermissions's request.
type ListAvailablePermissionsInput struct {
	Module string `query:"module"`
}

// ListAvailablePermissions returns the sections of the roles screen with the codes each one holds,
// optionally filtered by ?module=<module_name>. Labels are translated to the request language.
func (ctrl *PermissionController) ListAvailablePermissions(ctx context.Context, in *ListAvailablePermissionsInput) (*api.Out[models.PermissionCatalog], error) {
	c := api.Gin(ctx)
	lang := c.GetString("lang")
	sections := ctrl.service.ListSections(in.Module)
	for i := range sections {
		sections[i].Label = translateOr(lang, "permissions.section."+sections[i].Code, sections[i].Code)
	}
	return api.OK(models.PermissionCatalog{Sections: sections})
}
