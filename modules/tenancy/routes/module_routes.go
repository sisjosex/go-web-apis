package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

// RegisterModuleRoutes registers routes for tenant module management.
func RegisterModuleRoutes(
	router *gin.RouterGroup,
	moduleController *controllers.ModuleController,
	tenantService interfaces.TenantService,
	moduleService interfaces.ModuleService,
	jwtService services.JWTService,
) {
	// GET /modules — list available system modules (no tenant scope needed)
	api.Get(router, "/modules", moduleController.ListAvailableModules, api.Doc{}, authMiddleware.AuthMiddleware(jwtService))

	// Tenant-scoped module routes
	tenantRoutes := router.Group("/tenants/:tenant_slug/modules")
	tenantRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	tenantRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
	{
		// GET /tenants/:tenant_slug/modules
		api.Get(tenantRoutes, "", moduleController.GetTenantModules, api.Doc{})

		// Owner/admin only for mutations
		adminRoutes := tenantRoutes.Group("")
		adminRoutes.Use(tenantMiddleware.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin, coreModels.SystemRoleSuperAdmin))
		{
			// PUT /tenants/:tenant_slug/modules/:module_code
			api.Put(adminRoutes, "/{module_code}", moduleController.UpsertTenantModule, api.Doc{})
			// DELETE /tenants/:tenant_slug/modules/:module_code
			api.Delete(adminRoutes, "/{module_code}", moduleController.DisableTenantModule, api.Doc{})
		}
	}

	// PUT /tenants/:tenant_slug/vertical — the business type preset (TENANCY-006), owner/admin only.
	verticalRoutes := router.Group("/tenants/:tenant_slug/vertical")
	verticalRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	verticalRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
	verticalRoutes.Use(tenantMiddleware.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin, coreModels.SystemRoleSuperAdmin))
	// GET /tenants/:tenant_slug/setup — the first-run checklist (TENANCY-008), owner/admin only.
	setupRoutes := router.Group("/tenants/:tenant_slug/setup")
	setupRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	setupRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
	setupRoutes.Use(tenantMiddleware.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin, coreModels.SystemRoleSuperAdmin))
	api.Get(setupRoutes, "", moduleController.TenantSetup, api.Doc{Summary: "The first-run checklist (owner/admin only)", Description: "The business's data, the enabled modules' setup steps in the business types' order and the team, each with whether it is done, and the minutes left", Errors: []int{401, 403}})

	api.Put(verticalRoutes, "", moduleController.SetVerticals, api.Doc{Summary: "Pick the business types (owner/admin only)", Description: "Turns on the union of the types' modules and the suggested ones sent, stores the types, the main module and their capabilities, and with starter seeds their categories; never turns a module off", Errors: []int{401, 403, 422}})
}
