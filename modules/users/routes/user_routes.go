package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	tenancyMW "josex/web/modules/tenancy/middleware"
	"josex/web/modules/users/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes registers user management routes scoped to a tenant.
// Callers must pass the tenant middleware (TenantMiddlewareFromHeader when tenancy
// is enabled, or a no-op handler otherwise).
func RegisterUserRoutes(
	router *gin.RouterGroup,
	userController *controllers.UserController,
	userAuditController *controllers.UserAuditController,
	jwtService services.JWTService,
	tenantMiddleware gin.HandlerFunc,
) {
	userRoutes := router.Group("/users")
	userRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	userRoutes.Use(tenantMiddleware)
	userRoutes.Use(tenancyMW.RequireTenantRole("owner", "admin", "super_admin"))
	{
		userRoutes.GET("", userController.ListUsers)
		userRoutes.GET("/stats", userController.GetStats)
		userRoutes.GET("/audit", userAuditController.ListAudit)
		userRoutes.POST("", userController.Create)
		userRoutes.GET("/:id", userController.GetUserById)
		userRoutes.PUT("/:id", userController.Update)
		userRoutes.DELETE("/:id", userController.SoftDeleteUser)
		userRoutes.POST("/:id/reset-password", userController.ResetPassword)
	}
}
