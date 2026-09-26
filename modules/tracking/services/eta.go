package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	coreServices "josex/web/modules/core/services"
	geoInterfaces "josex/web/modules/geo/interfaces"
	geoModels "josex/web/modules/geo/models"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

const (
	// etaTTL is D1's cache: a watched trip costs Valhalla one call per 30 s, whoever asks.
	etaTTL = 30 * time.Second
	// etaMaxStops keeps one route call inside Valhalla's location limit; stops past it get no ETA.
	etaMaxStops = 19
)

// TripEta is the ETA of a trip's pending stops and when it was computed — the gateway compares the
// latter to know whether it has already sent this estimate.
type TripEta struct {
	ComputedAt time.Time        `json:"computed_at"`
	Eta        []models.StopEta `json:"eta"`
}

// EtaService computes ETAs on demand (TRACK-025 D1): only for a trip someone is looking at, cached
// eta:{trip} in Valkey for 30 s so every API instance and gateway shares one computation.
type EtaService struct {
	router geoInterfaces.Router
	geo    geoInterfaces.GeoService
	valkey coreServices.ValkeyService
}

// NewEtaService takes the geo router (one route call through every pending stop) and the geo
// service (its per-leg fallback when the router is down). valkey nil computes on every call.
func NewEtaService(router geoInterfaces.Router, geo geoInterfaces.GeoService, valkey coreServices.ValkeyService) *EtaService {
	return &EtaService{router: router, geo: geo, valkey: valkey}
}

func etaKey(tripID uuid.UUID) string { return "eta:" + tripID.String() }

// Cached answers the trip's ETA while its 30 s entry lives, without touching the database.
func (s *EtaService) Cached(ctx context.Context, tripID uuid.UUID) (TripEta, bool) {
	if s.valkey == nil {
		return TripEta{}, false
	}
	raw, err := s.valkey.Client().Get(ctx, etaKey(tripID)).Bytes()
	if err != nil {
		return TripEta{}, false
	}
	var cached TripEta
	return cached, json.Unmarshal(raw, &cached) == nil
}

// ForTrip answers the ETA from the vehicle's position to each pending stop, in order. No position or
// no pending stop: an empty ETA, nothing cached.
func (s *EtaService) ForTrip(ctx context.Context, tripID uuid.UUID, position *models.LivePosition, pending []models.PendingStop) (TripEta, error) {
	if position == nil || len(pending) == 0 {
		return TripEta{Eta: []models.StopEta{}}, nil
	}
	if cached, ok := s.Cached(ctx, tripID); ok {
		return cached, nil
	}

	if len(pending) > etaMaxStops {
		pending = pending[:etaMaxStops]
	}
	legs, err := s.legs(ctx, position, pending)
	if err != nil {
		return TripEta{}, err
	}
	now := time.Now().UTC()
	out := TripEta{ComputedAt: now, Eta: make([]models.StopEta, len(legs))}
	elapsed, source := 0.0, geoModels.EstimateValhalla
	for i, stop := range pending[:len(legs)] {
		elapsed += legs[i].DurationS
		source = worstSource(source, legs[i].EstimateSource)
		out.Eta[i] = models.StopEta{TripStopID: stop.TripStopID, EtaAt: now.Add(time.Duration(elapsed) * time.Second), EstimateSource: source}
	}
	if s.valkey != nil {
		if raw, err := json.Marshal(out); err == nil {
			_ = s.valkey.Client().Set(ctx, etaKey(tripID), raw, etaTTL).Err()
		}
	}
	return out, nil
}

// legs is the duration of each stretch position → stop 1 → stop 2 …: one Valhalla route through all
// of them, or, when it cannot answer, geo's own ETA leg by leg (observed durations, then the straight
// line), each flagged by its source. It may answer fewer legs than stops: none past a stop no road
// reaches.
func (s *EtaService) legs(ctx context.Context, position *models.LivePosition, pending []models.PendingStop) ([]geoModels.Eta, error) {
	points := make([]geoModels.LatLng, 0, len(pending)+1)
	points = append(points, geoModels.LatLng{Lat: position.Lat, Lng: position.Lng})
	for _, p := range pending {
		points = append(points, geoModels.LatLng{Lat: p.Lat, Lng: p.Lng})
	}

	if s.router != nil {
		route, err := s.router.Route(ctx, points, "bus")
		if err == nil && len(route.Legs) == len(pending) {
			legs := make([]geoModels.Eta, len(route.Legs))
			for i, l := range route.Legs {
				legs[i] = geoModels.Eta{DurationS: l.DurationS, DistanceM: l.DistanceM, EstimateSource: geoModels.EstimateValhalla}
			}
			return legs, nil
		}
		if err != nil && !errors.Is(err, geoInterfaces.ErrRouteUnavailable) && !errors.Is(err, geoInterfaces.ErrNoRoute) {
			return nil, err
		}
	}

	legs := make([]geoModels.Eta, len(pending))
	for i := range pending {
		from, to := points[i], points[i+1]
		eta, err := s.geo.Eta(ctx, &geoModels.EtaRequest{From: &from, To: &to, Costing: "bus"})
		if errors.Is(err, geoInterfaces.ErrNoRoute) {
			// No road reaches this stop: it and the stops after it get no estimate.
			return legs[:i], nil
		}
		if err != nil {
			return nil, err
		}
		legs[i] = *eta
	}
	return legs, nil
}

// worstSource is the less reliable of two estimate sources: a stop is only as good as the legs
// before it.
func worstSource(a, b string) string {
	rank := map[string]int{geoModels.EstimateValhalla: 0, geoModels.EstimateHistorical: 1, geoModels.EstimateFallback: 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
