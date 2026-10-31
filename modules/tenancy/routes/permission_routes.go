package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
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
	api.Get(router, "/permissions", permController.ListAvailablePermissions, api.Doc{Summary: "List the roles screen's sections", Description: "Each section with its read and write permission codes: \"Ver\" is the read ones, \"Editar\" all of them"}, auth)
}
