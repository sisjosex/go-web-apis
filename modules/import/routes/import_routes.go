package routes

import (
	"github.com/gin-gonic/gin"

	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/import/controllers"
)

// RegisterImportRoutes registers the generic import endpoints, scoped to a tenant.
// Callers pass the tenant middleware (TenantMiddlewareFromHeader when tenancy is
// enabled, or a no-op handler otherwise).
func RegisterImportRoutes(
	router *gin.RouterGroup,
	importController *controllers.ImportController,
	jwtService services.JWTService,
	tenantMiddleware gin.HandlerFunc,
) {
	importRoutes := router.Group("/import")
	importRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	importRoutes.Use(tenantMiddleware)
	{
		importRoutes.GET("/meta", importController.Meta)
		importRoutes.GET("/template", importController.Template)
		importRoutes.POST("/validate", importController.Validate)
		importRoutes.POST("", importController.Import)
	}
}
