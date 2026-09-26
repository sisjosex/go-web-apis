package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	geoConfig "josex/web/modules/geo/config"
	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"

	"github.com/sony/gobreaker/v2"
)

// Breaker policy (D5): five failures in a row open it for 30 s; one trial call then decides.
const (
	breakerFailures = 5
	breakerOpenFor  = 30 * time.Second
)

// NewRouter picks the routing adapter. Valhalla is the only one.
func NewRouter(cfg *geoConfig.GeoConfig) interfaces.Router {
	return newValhallaProvider(cfg.ValhallaURL, cfg.Timeout)
}

type valhallaProvider struct {
	baseURL string
	client  *http.Client
	breaker *gobreaker.CircuitBreaker[*models.Route]
}

func newValhallaProvider(baseURL string, timeout time.Duration) *valhallaProvider {
	return &valhallaProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
		breaker: gobreaker.NewCircuitBreaker[*models.Route](gobreaker.Settings{
			Name:        "valhalla",
			Timeout:     breakerOpenFor,
			ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= breakerFailures },
			// "No route" is Valhalla working; only outages count toward opening.
			IsSuccessful: func(err error) bool { return err == nil || errors.Is(err, interfaces.ErrNoRoute) },
		}),
	}
}

type valhallaLocation struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type valhallaRequest struct {
	Locations      []valhallaLocation `json:"locations"`
	Costing        string             `json:"costing"`
	DirectionsType string             `json:"directions_type"`
	Units          string             `json:"units"`
}

type valhallaSummary struct {
	Time   float64 `json:"time"`   // seconds
	Length float64 `json:"length"` // kilometres
}

type valhallaResponse struct {
	Trip struct {
		Summary valhallaSummary `json:"summary"`
		Legs    []struct {
			Shape   string          `json:"shape"`
			Summary valhallaSummary `json:"summary"`
		} `json:"legs"`
	} `json:"trip"`
}

func (p *valhallaProvider) Route(ctx context.Context, locations []models.LatLng, costing string) (*models.Route, error) {
	route, err := p.breaker.Execute(func() (*models.Route, error) { return p.route(ctx, locations, costing) })
	if err == nil || errors.Is(err, interfaces.ErrNoRoute) {
		return route, err
	}
	return nil, fmt.Errorf("%w: %w", interfaces.ErrRouteUnavailable, err)
}

func (p *valhallaProvider) route(ctx context.Context, locations []models.LatLng, costing string) (*models.Route, error) {
	body := valhallaRequest{Costing: costing, DirectionsType: "none", Units: "kilometers"}
	for _, l := range locations {
		body.Locations = append(body.Locations, valhallaLocation{Lat: l.Lat, Lon: l.Lng})
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/route", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	switch {
	case res.StatusCode >= http.StatusInternalServerError:
		return nil, fmt.Errorf("valhalla answered %d", res.StatusCode)
	case res.StatusCode >= http.StatusBadRequest:
		// Unreachable or off-map locations. Valhalla's error_code says which; the client needs only "no route".
		_, _ = io.Copy(io.Discard, res.Body)
		return nil, interfaces.ErrNoRoute
	}

	var out valhallaResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return toRoute(&out)
}

func toRoute(out *valhallaResponse) (*models.Route, error) {
	route := &models.Route{
		DurationS: out.Trip.Summary.Time,
		DistanceM: out.Trip.Summary.Length * 1000,
		Legs:      make([]models.RouteLeg, 0, len(out.Trip.Legs)),
	}
	shapes := make([]string, 0, len(out.Trip.Legs))
	for _, leg := range out.Trip.Legs {
		route.Legs = append(route.Legs, models.RouteLeg{
			Polyline6: leg.Shape,
			DurationS: leg.Summary.Time,
			DistanceM: leg.Summary.Length * 1000,
		})
		shapes = append(shapes, leg.Shape)
	}
	joined, err := joinPolylines(shapes)
	if err != nil {
		return nil, err
	}
	route.Polyline6 = joined
	return route, nil
}
