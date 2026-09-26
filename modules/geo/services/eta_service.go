package services

import (
	"context"
	"errors"
	"math"

	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"
)

const (
	// detourFactor turns a straight line into a plausible street distance (D5).
	detourFactor = 1.3
	// walkingKmh is the pedestrian fallback: a walking pace is not a deployment setting.
	walkingKmh   = 5.0
	earthRadiusM = 6371000.0
	kmhToMs      = 1000.0 / 3600.0
)

// Eta tries each source in order (D5): Valhalla, then observed durations, then the straight line at
// a fixed speed. Only an outage falls through; "no route" is an answer and is returned as such.
func (s *geoService) Eta(ctx context.Context, req *models.EtaRequest) (*models.Eta, error) {
	from, to := *req.From, *req.To

	route, err := s.router.Route(ctx, []models.LatLng{from, to}, req.Costing)
	if err == nil {
		return &models.Eta{DurationS: route.DurationS, DistanceM: route.DistanceM, EstimateSource: models.EstimateValhalla}, nil
	}
	if !errors.Is(err, interfaces.ErrRouteUnavailable) {
		return nil, err
	}

	distance := haversineM(from, to) * detourFactor
	if s.historical != nil {
		if seconds, ok, herr := s.historical.Duration(ctx, from, to, req.Costing); herr == nil && ok {
			return &models.Eta{DurationS: seconds, DistanceM: distance, EstimateSource: models.EstimateHistorical}, nil
		}
	}

	kmh := s.busKmh
	if req.Costing == "pedestrian" {
		kmh = walkingKmh
	}
	return &models.Eta{
		DurationS:      math.Round(distance / (kmh * kmhToMs)),
		DistanceM:      math.Round(distance),
		EstimateSource: models.EstimateFallback,
	}, nil
}

func haversineM(a, b models.LatLng) float64 {
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLng := (b.Lng - a.Lng) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}
