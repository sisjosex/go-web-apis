package routes

import (
	"josex/web/modules/core/api"
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

	api.Get(r, "/geocode", ctrl.Geocode, api.Doc{Summary: "Search addresses: streets, places and POIs by name", Description: "Near lat/lng, a close match outranks a better one far away.", Errors: []int{400}})
	api.Get(r, "/reverse", ctrl.Reverse, api.Doc{Summary: "The nearest known place to a point, within 500 m", Errors: []int{400, 404}})
	api.Post(r, "/route", ctrl.Route, api.Doc{Summary: "The route through the locations in order, by bus or on foot", Errors: []int{400, 422, 503}})
	api.Post(r, "/eta", ctrl.Eta, api.Doc{Summary: "Travel time between two points; still answers when routing is down", Description: "estimate_source says how: valhalla, historical, or fallback (straight line at a fixed speed).", Errors: []int{400, 422}})
}
