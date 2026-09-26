package routes

import (
	"josex/web/modules/geo/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterGeoRoutes mounts /api/v1/geo behind auth and the tenant chain. Reads only, and none of it
// tenant data, so no per-route permission.
func RegisterGeoRoutes(
	apiV1 *gin.RouterGroup,
	ctrl *controllers.GeoController,
	authMiddleware gin.HandlerFunc,
	tenantMiddleware gin.HandlerFunc,
) {
	r := apiV1.Group("/geo", authMiddleware, tenantMiddleware)

	r.GET("/geocode", ctrl.Geocode)
	r.GET("/reverse", ctrl.Reverse)
	r.POST("/route", ctrl.Route)
	r.POST("/eta", ctrl.Eta)
}
