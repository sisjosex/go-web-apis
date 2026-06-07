package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	coreMiddleware "josex/web/modules/core/middleware"
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
	tenantRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	{
		// ========================================
		// Tenant Management
		// ========================================

		// Create tenant (super_admin only - full control, custom database_url allowed)
		adminOnlyRoutes := tenantRoutes.Group("")
		adminOnlyRoutes.Use(coreMiddleware.RequireSystemRole("super_admin")) // 🔒 PROTECTED
		{
			adminOnlyRoutes.POST("", tenantController.CreateTenant)
		}

		// Self-service tenant creation (authenticated users with plan limits)
		tenantRoutes.POST("/self-service", tenantController.CreateTenantSelfService)

		// Get user's accessible tenants
		tenantRoutes.GET("/me", tenantController.GetUserTenants)
		tenantRoutes.GET("/my-tenants", tenantController.GetUserTenants) // kept for backward compatibility

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
				adminRoutes.PATCH("/users/:user_id/role", tenantController.UpdateUserRole)
			}

			// Run migrations on tenant database (super_admin only)
			superAdminRoutes := tenantScopedRoutes.Use(coreMiddleware.RequireSystemRole("super_admin"))
			{
				superAdminRoutes.POST("/migrate", tenantController.RunTenantMigrations)
			}
		}
	}
}
