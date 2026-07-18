package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/tenancy/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterPermissionRoutes registers the permission catalog route.
// Call this alongside RegisterTenantRoutes in routes/routes.go.
func RegisterPermissionRoutes(
	router *gin.RouterGroup,
	permController *controllers.PermissionController,
	jwtService services.JWTService,
) {
	auth := authMiddleware.AuthMiddleware(jwtService)

	// GET /api/v1/permissions?module=inventory — list available permissions (auth required)
	router.GET("/permissions", auth, permController.ListAvailablePermissions)
}
