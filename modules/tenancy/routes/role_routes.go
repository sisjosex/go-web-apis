package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoleRoutes registers tenant role management routes.
// Call this alongside RegisterTenantRoutes and RegisterPermissionRoutes in routes/routes.go.
func RegisterRoleRoutes(
	router *gin.RouterGroup,
	roleController *controllers.RoleController,
	tenantService interfaces.TenantService,
	jwtService services.JWTService,
) {
	auth := authMiddleware.AuthMiddleware(jwtService)

	tenantRoutes := router.Group("/tenants/:tenant_slug")
	tenantRoutes.Use(auth, tenantMiddleware.TenantMiddleware(tenantService))
	{
		// Any tenant member can list roles or view a single role
		tenantRoutes.GET("/roles", roleController.ListRoles)
		tenantRoutes.GET("/roles/:role_id", roleController.GetRole)

		// owner/admin only: create, update, delete roles and manage permissions
		adminRoutes := tenantRoutes.Group("")
		adminRoutes.Use(tenantMiddleware.RequireTenantRole("owner", "admin"))
		{
			adminRoutes.POST("/roles", roleController.CreateRole)
			adminRoutes.PATCH("/roles/:role_id", roleController.UpdateRole)
			adminRoutes.DELETE("/roles/:role_id", roleController.DeleteRole)
			adminRoutes.PUT("/roles/:role_id/permissions", roleController.SetRolePermissions)

			// User role assignment/revocation
			adminRoutes.POST("/users/:user_id/roles/:role_id", roleController.AssignUserRole)
			adminRoutes.DELETE("/users/:user_id/roles/:role_id", roleController.RevokeUserRole)
		}

		// Any tenant member can list a user's roles
		tenantRoutes.GET("/users/:user_id/roles", roleController.ListUserRoles)
	}
}
