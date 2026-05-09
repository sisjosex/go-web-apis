package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"

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
	router.GET("/modules", authMiddleware.AuthMiddleware(jwtService), moduleController.ListAvailableModules)

	// Tenant-scoped module routes
	tenantRoutes := router.Group("/tenants/:tenant_slug/modules")
	tenantRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	tenantRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
	{
		// GET /tenants/:tenant_slug/modules
		tenantRoutes.GET("", moduleController.GetTenantModules)

		// Owner/admin only for mutations
		adminRoutes := tenantRoutes.Group("")
		adminRoutes.Use(tenantMiddleware.RequireTenantRole("owner", "admin", "super_admin"))
		{
			// PUT /tenants/:tenant_slug/modules/:module_code
			adminRoutes.PUT("/:module_code", moduleController.UpsertTenantModule)
			// DELETE /tenants/:tenant_slug/modules/:module_code
			adminRoutes.DELETE("/:module_code", moduleController.DisableTenantModule)
		}
	}
}
