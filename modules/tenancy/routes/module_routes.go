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
}
