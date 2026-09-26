package controllers

import (
	"errors"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	geoErrors "josex/web/modules/geo/errors"
	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"

	"github.com/gin-gonic/gin"
)

// GeoController serves address search and routing (INFRA-003). Nothing here is tenant data: OSM is
// the same for everyone, so the tenant chain only gates who may call.
type GeoController struct {
	service interfaces.GeoService
}

func NewGeoController(service interfaces.GeoService) *GeoController {
	return &GeoController{service: service}
}

// Geocode godoc
// @Summary Search addresses: streets, places and POIs by name
// @Description Near lat/lng, a close match outranks a better one far away.
// @Tags Geo
// @Produce json
// @Param q query string true "What to look for, at least 2 characters"
// @Param lat query number false "Rank matches near this point first (with lng)"
// @Param lng query number false "Rank matches near this point first (with lat)"
// @Param limit query int false "1-20, default 8"
// @Success 200 {object} models.GeocodeResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /geo/geocode [get]
// @Security ApiKeyAuth
func (ctrl *GeoController) Geocode(c *gin.Context) {
	var q models.GeocodeQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	if (q.Lat == nil) != (q.Lng == nil) {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, geoErrors.PointIncomplete))
		return
	}

	places, err := ctrl.service.Search(c.Request.Context(), &q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, geoErrors.GeocodeFailed))
		return
	}
	c.JSON(http.StatusOK, models.GeocodeResponse{Results: places})
}

// Reverse godoc
// @Summary The nearest known place to a point, within 500 m
// @Tags Geo
// @Produce json
// @Param lat query number true "Latitude"
// @Param lng query number true "Longitude"
// @Success 200 {object} models.Place
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /geo/reverse [get]
// @Security ApiKeyAuth
func (ctrl *GeoController) Reverse(c *gin.Context) {
	var q models.ReverseQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	place, err := ctrl.service.Reverse(c.Request.Context(), *q.Lat, *q.Lng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, geoErrors.ReverseFailed))
		return
	}
	if place == nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, geoErrors.ReverseNotFound))
		return
	}
	c.JSON(http.StatusOK, place)
}

// Route godoc
// @Summary The route through the locations in order, by bus or on foot
// @Tags Geo
// @Accept json
// @Produce json
// @Param request body models.RouteRequest true "2-20 locations and a costing"
// @Success 200 {object} models.Route
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 422 {object} coreErrors.ErrorResponse "no route joins the locations"
// @Failure 503 {object} coreErrors.ErrorResponse "routing unavailable, retry later"
// @Router /geo/route [post]
// @Security ApiKeyAuth
func (ctrl *GeoController) Route(c *gin.Context) {
	var req models.RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	route, err := ctrl.service.Route(c.Request.Context(), &req)
	if err != nil {
		ctrl.routingError(c, err, geoErrors.RouteUnavailable)
		return
	}
	c.JSON(http.StatusOK, route)
}

// Eta godoc
// @Summary Travel time between two points; still answers when routing is down
// @Description estimate_source says how: valhalla, historical, or fallback (straight line at a fixed speed).
// @Tags Geo
// @Accept json
// @Produce json
// @Param request body models.EtaRequest true "from, to and a costing"
// @Success 200 {object} models.Eta
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 422 {object} coreErrors.ErrorResponse "no route joins the points"
// @Router /geo/eta [post]
// @Security ApiKeyAuth
func (ctrl *GeoController) Eta(c *gin.Context) {
	var req models.EtaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	eta, err := ctrl.service.Eta(c.Request.Context(), &req)
	if err != nil {
		ctrl.routingError(c, err, geoErrors.EtaFailed)
		return
	}
	c.JSON(http.StatusOK, eta)
}

// routingError maps the routing port's errors; anything else is fallbackCode with a 500.
func (ctrl *GeoController) routingError(c *gin.Context, err error, fallbackCode string) {
	switch {
	case errors.Is(err, interfaces.ErrNoRoute):
		c.JSON(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, geoErrors.RouteNotFound))
	case errors.Is(err, interfaces.ErrRouteUnavailable):
		c.JSON(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, geoErrors.RouteUnavailable))
	default:
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, fallbackCode))
	}
}
