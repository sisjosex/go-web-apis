package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterPermissionRoutes registers permission management routes.
// Call this alongside RegisterTenantRoutes in routes/routes.go.
func RegisterPermissionRoutes(
	router *gin.RouterGroup,
	permController *controllers.PermissionController,
	tenantService interfaces.TenantService,
	jwtService services.JWTService,
) {
	auth := authMiddleware.AuthMiddleware(jwtService)

	// GET /api/v1/permissions?module=inventory — list available permissions (auth required)
	router.GET("/permissions", auth, permController.ListAvailablePermissions)

	// Tenant-scoped permission routes — require tenant membership
	tenantRoutes := router.Group("/tenants/:tenant_slug")
	tenantRoutes.Use(auth, tenantMiddleware.TenantMiddleware(tenantService))
	{
		// Any tenant member can view their own permissions
		tenantRoutes.GET("/my-permissions", permController.GetMyPermissions)

		// owner/admin only: manage user permissions
		adminRoutes := tenantRoutes.Group("/users/:user_id/permissions")
		adminRoutes.Use(tenantMiddleware.RequireTenantRole("owner", "admin"))
		{
			adminRoutes.GET("", permController.ListUserPermissions)
			adminRoutes.POST("", permController.AssignPermission)
			adminRoutes.DELETE("", permController.RevokePermission)
		}
	}
}
