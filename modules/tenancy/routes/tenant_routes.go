package routes

import (
	"josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterTenantRoutes registers all routes for tenancy module
func RegisterTenantRoutes(
	router *gin.RouterGroup,
	tenantController *controllers.TenantController,
	tenantService interfaces.TenantService,
	jwtService services.JWTService,
) {
	// All tenant routes require authentication
	tenantRoutes := router.Group("/tenants")
	tenantRoutes.Use(middleware.AuthMiddleware(jwtService))
	{
		// ========================================
		// Tenant Management (System Admin)
		// ========================================

		// Create tenant (system admin only - TODO: add admin middleware)
		tenantRoutes.POST("", tenantController.CreateTenant)

		// Get user's accessible tenants
		tenantRoutes.GET("/my-tenants", tenantController.GetUserTenants)

		// ========================================
		// Tenant-Scoped Routes (require tenant access)
		// ========================================
		// Pattern: /tenants/:tenant_slug/*
		tenantScopedRoutes := tenantRoutes.Group("/:tenant_slug")
		tenantScopedRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
		{
			// Tenant information (all members)
			// tenantScopedRoutes.GET("", tenantController.GetTenantInfo)

			// Update tenant (owner/admin only)
			adminRoutes := tenantScopedRoutes.Use(tenantMiddleware.RequireTenantRole("owner", "admin"))
			{
				adminRoutes.PATCH("", tenantController.UpdateTenant)
				adminRoutes.POST("/users", tenantController.AddUserToTenant)
				adminRoutes.DELETE("/users/:user_id", tenantController.RemoveUserFromTenant)
			}
		}
	}
}
