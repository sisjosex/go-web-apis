package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
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

// GeocodeInput is Geocode's request.
type GeocodeInput struct {
	Query api.Query[models.GeocodeQuery]
}

// Geocode is GET /geocode: Search addresses: streets, places and POIs by name.
func (ctrl *GeoController) Geocode(ctx context.Context, in *GeocodeInput) (*api.Out[models.GeocodeResponse], error) {
	c := api.Gin(ctx)
	var q models.GeocodeQuery
	if err := in.Query.Into(ctx, &q); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
	}
	if (q.Lat == nil) != (q.Lng == nil) {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, geoErrors.PointIncomplete))
	}

	places, err := ctrl.service.Search(c.Request.Context(), &q)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, geoErrors.GeocodeFailed))
	}
	return api.OK(models.GeocodeResponse{Results: places})
}

// ReverseInput is Reverse's request.
type ReverseInput struct {
	Query api.Query[models.ReverseQuery]
}

// Reverse is GET /reverse: The nearest known place to a point, within 500 m.
func (ctrl *GeoController) Reverse(ctx context.Context, in *ReverseInput) (*api.Out[*models.Place], error) {
	c := api.Gin(ctx)
	var q models.ReverseQuery
	if err := in.Query.Into(ctx, &q); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
	}

	place, err := ctrl.service.Reverse(c.Request.Context(), *q.Lat, *q.Lng)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, geoErrors.ReverseFailed))
	}
	if place == nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, geoErrors.ReverseNotFound))
	}
	return api.OK(place)
}

// RouteInput is Route's request.
type RouteInput struct {
	JSON api.Body[models.RouteRequest]
}

// Route is POST /route: The route through the locations in order, by bus or on foot.
func (ctrl *GeoController) Route(ctx context.Context, in *RouteInput) (*api.Out[*models.Route], error) {
	c := api.Gin(ctx)
	var req models.RouteRequest
	if err := in.JSON.Into(ctx, &req); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
	}

	route, err := ctrl.service.Route(c.Request.Context(), &req)
	if err != nil {
		return nil, ctrl.routingError(c, err, geoErrors.RouteUnavailable)
	}
	return api.OK(route)
}

// EtaInput is Eta's request.
type EtaInput struct {
	JSON api.Body[models.EtaRequest]
}

// Eta is POST /eta: Travel time between two points; still answers when routing is down.
func (ctrl *GeoController) Eta(ctx context.Context, in *EtaInput) (*api.Out[*models.Eta], error) {
	c := api.Gin(ctx)
	var req models.EtaRequest
	if err := in.JSON.Into(ctx, &req); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, geoErrors.ValidationFailed, utils.ExtractValidationError(c, err)))
	}

	eta, err := ctrl.service.Eta(c.Request.Context(), &req)
	if err != nil {
		return nil, ctrl.routingError(c, err, geoErrors.EtaFailed)
	}
	return api.OK(eta)
}

// routingError maps the routing port's errors; anything else is fallbackCode with a 500.
func (ctrl *GeoController) routingError(c *gin.Context, err error, fallbackCode string) error {
	switch {
	case errors.Is(err, interfaces.ErrNoRoute):
		return api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, geoErrors.RouteNotFound))
	case errors.Is(err, interfaces.ErrRouteUnavailable):
		return api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, geoErrors.RouteUnavailable))
	default:
		return api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, fallbackCode))
	}
}
