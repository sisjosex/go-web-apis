package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	coreMiddleware "josex/web/modules/core/middleware"
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

// RegisterTenantRoutes registers all routes for tenancy module
func RegisterTenantRoutes(
	router *gin.RouterGroup,
	tenantController *controllers.TenantController,
	settingsController *controllers.SettingsController,
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
		adminOnlyRoutes.Use(coreMiddleware.RequireSystemRole(coreModels.SystemRoleSuperAdmin)) // 🔒 PROTECTED
		{
			api.Post(adminOnlyRoutes, "", tenantController.CreateTenant, api.Doc{Summary: "Create a new tenant (authenticated users)", Description: "Create a new tenant with optional custom database URL. Creator becomes owner automatically.", Status: 201, Errors: []int{400}})
		}

		// Self-service tenant creation (authenticated users with plan limits)
		api.Post(tenantRoutes, "/self-service", tenantController.CreateTenantSelfService, api.Doc{Summary: "Create tenant (self-service with subscription limits)", Description: "Allow authenticated users to create tenants with limits based on their subscription plan", Status: 201, Errors: []int{400, 403}})

		// Get user's accessible tenants
		api.Get(tenantRoutes, "/me", tenantController.GetUserTenants, api.Doc{Summary: "Get user's accessible tenants", Description: "Returns list of tenants that the authenticated user can access", Errors: []int{401}})
		api.Get(tenantRoutes, "/my-tenants", tenantController.GetUserTenants, api.Doc{Summary: "Get user's accessible tenants", Description: "Returns list of tenants that the authenticated user can access", Errors: []int{401}}) // kept for backward compatibility

		// ========================================
		// Tenant-Scoped Routes (require tenant access)
		// ========================================
		// Pattern: /tenants/:tenant_slug/*
		tenantScopedRoutes := tenantRoutes.Group("/:tenant_slug")
		tenantScopedRoutes.Use(tenantMiddleware.TenantMiddleware(tenantService))
		{
			// Tenant information (all members)
			// tenantScopedRoutes.GET("", tenantController.GetTenantInfo)
			api.Get(tenantScopedRoutes, "/settings", settingsController.GetSettings, api.Doc{Summary: "Get the workspace's settings", Description: "Every setting its enabled modules declare, with the default where it never set one, and each key's type and options (TENANCY-007)", Errors: []int{401}})

			// Update tenant (owner/admin only)
			// Use adds to tenantScopedRoutes itself: every route registered on it from here carries it.
			tenantScopedRoutes.Use(tenantMiddleware.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin))
			adminRoutes := tenantScopedRoutes
			{
				api.Patch(adminRoutes, "", tenantController.UpdateTenant, api.Doc{Summary: "Update tenant information (owner/admin only)", Description: "Update tenant name, database URL, schema, or settings", Errors: []int{400}})
				api.Patch(adminRoutes, "/settings", settingsController.UpdateSettings, api.Doc{Summary: "Change the workspace's settings (owner/admin only)", Description: "Merges the keys sent, keeping the rest; an undeclared key or a value its type refuses answers 422", Errors: []int{401, 403, 422}})
				api.Post(adminRoutes, "/users", tenantController.AddUserToTenant, api.Doc{Summary: "Add a user to tenant (owner/admin only)", Description: "Add a user to the tenant with specified role", Errors: []int{400}})
				api.Delete(adminRoutes, "/users/{user_id}", tenantController.RemoveUserFromTenant, api.Doc{Summary: "Remove a user from tenant (owner/admin only)", Description: "Remove a user from the tenant", Errors: []int{400}})
				api.Patch(adminRoutes, "/users/{user_id}/role", tenantController.UpdateUserRole, api.Doc{Summary: "Update a user's role in the tenant (owner/admin only)", Description: "Change the role of a tenant member. Only owner can assign the owner role.", Status: 204, Errors: []int{400, 403}})
			}

			// Run migrations on tenant database (super_admin only)
			// Use adds to tenantScopedRoutes itself: every route registered on it from here carries it.
			tenantScopedRoutes.Use(coreMiddleware.RequireSystemRole(coreModels.SystemRoleSuperAdmin))
			superAdminRoutes := tenantScopedRoutes
			{
				api.Post(superAdminRoutes, "/migrate", tenantController.RunTenantMigrations, api.Doc{Summary: "Run migrations on tenant database", Description: "Execute all pending migrations on a tenant's custom database (admin/super_admin only)", Errors: []int{400, 403, 404}})
			}
		}
	}
}
