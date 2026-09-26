package services

import (
	"context"

	geoConfig "josex/web/modules/geo/config"
	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"
	"josex/web/modules/geo/repositories"
)

// defaultSearchLimit is a dropdown's worth of suggestions.
const defaultSearchLimit = 8

type geoService struct {
	places     *repositories.PlacesRepository
	router     interfaces.Router
	historical interfaces.HistoricalDurations // nil until TRACK-010
	busKmh     float64
}

// NewGeoService wires the address search and the routing port. historical may be nil.
func NewGeoService(
	places *repositories.PlacesRepository,
	router interfaces.Router,
	historical interfaces.HistoricalDurations,
	cfg *geoConfig.GeoConfig,
) interfaces.GeoService {
	return &geoService{places: places, router: router, historical: historical, busKmh: float64(cfg.FallbackSpeedKmh)}
}

func (s *geoService) Search(ctx context.Context, q *models.GeocodeQuery) ([]models.Place, error) {
	limit := q.Limit
	if limit == 0 {
		limit = defaultSearchLimit
	}
	return s.places.Search(ctx, q.Q, q.Lat, q.Lng, limit)
}

func (s *geoService) Reverse(ctx context.Context, lat, lng float64) (*models.Place, error) {
	return s.places.Reverse(ctx, lat, lng)
}

func (s *geoService) Route(ctx context.Context, req *models.RouteRequest) (*models.Route, error) {
	return s.router.Route(ctx, req.Locations, req.Costing)
}
