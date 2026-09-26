package interfaces

import (
	"context"

	"josex/web/modules/geo/models"
)

// GeoService is what the geo controller calls.
type GeoService interface {
	Search(ctx context.Context, q *models.GeocodeQuery) ([]models.Place, error)
	// Reverse returns nil, nil when no place is near enough.
	Reverse(ctx context.Context, lat, lng float64) (*models.Place, error)
	Route(ctx context.Context, req *models.RouteRequest) (*models.Route, error)
	// Eta answers even with the router down, flagged by EstimateSource; ErrNoRoute passes through.
	Eta(ctx context.Context, req *models.EtaRequest) (*models.Eta, error)
}
