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
	// One breaker for every call: route and trace fail together when Valhalla does.
	breaker *gobreaker.CircuitBreaker[any]
}

func newValhallaProvider(baseURL string, timeout time.Duration) *valhallaProvider {
	return &valhallaProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
		breaker: gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
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
	res, err := p.breaker.Execute(func() (any, error) { return p.route(ctx, locations, costing) })
	if err == nil || errors.Is(err, interfaces.ErrNoRoute) {
		route, _ := res.(*models.Route)
		return route, err
	}
	return nil, fmt.Errorf("%w: %w", interfaces.ErrRouteUnavailable, err)
}

func (p *valhallaProvider) route(ctx context.Context, locations []models.LatLng, costing string) (*models.Route, error) {
	body := valhallaRequest{Costing: costing, DirectionsType: "none", Units: "kilometers"}
	for _, l := range locations {
		body.Locations = append(body.Locations, valhallaLocation{Lat: l.Lat, Lon: l.Lng})
	}
	var out valhallaResponse
	if err := p.post(ctx, "/route", body, &out); err != nil {
		return nil, err
	}
	return toRoute(&out)
}

// maxTraceShape keeps a trace under Valhalla's max_shape (16 000) with room to spare: a longer path
// is thinned evenly, first and last point kept.
const maxTraceShape = 5000

type valhallaTraceRequest struct {
	Shape      []valhallaLocation  `json:"shape"`
	Costing    string              `json:"costing"`
	ShapeMatch string              `json:"shape_match"`
	Filters    valhallaTraceFilter `json:"filters"`
}

type valhallaTraceFilter struct {
	Attributes []string `json:"attributes"`
	Action     string   `json:"action"`
}

type valhallaTraceResponse struct {
	Shape string `json:"shape"`
}

func (p *valhallaProvider) Trace(ctx context.Context, points []models.LatLng, costing string) (*models.Trace, error) {
	res, err := p.breaker.Execute(func() (any, error) { return p.trace(ctx, points, costing) })
	if err == nil || errors.Is(err, interfaces.ErrNoRoute) {
		trace, _ := res.(*models.Trace)
		return trace, err
	}
	return nil, fmt.Errorf("%w: %w", interfaces.ErrRouteUnavailable, err)
}

// trace asks trace_attributes for the matched shape only; its length is measured on the line itself,
// which counts the first and last edges only as far as the vehicle drove them.
func (p *valhallaProvider) trace(ctx context.Context, points []models.LatLng, costing string) (*models.Trace, error) {
	body := valhallaTraceRequest{
		Costing:    costing,
		ShapeMatch: "map_snap",
		Filters:    valhallaTraceFilter{Attributes: []string{"shape"}, Action: "include"},
	}
	for _, l := range thin(points, maxTraceShape) {
		body.Shape = append(body.Shape, valhallaLocation{Lat: l.Lat, Lon: l.Lng})
	}
	var out valhallaTraceResponse
	if err := p.post(ctx, "/trace_attributes", body, &out); err != nil {
		return nil, err
	}
	decoded, err := decodePolyline(out.Shape)
	if err != nil {
		return nil, err
	}
	return &models.Trace{Polyline6: out.Shape, DistanceM: lineLengthM(decoded)}, nil
}

// post sends one request and decodes the answer: a 5xx or a transport error is an outage, a 4xx is
// ErrNoRoute (unreachable or off-map locations — Valhalla's error_code says which; the caller needs
// only "no route").
func (p *valhallaProvider) post(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	switch {
	case res.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf("valhalla answered %d", res.StatusCode)
	case res.StatusCode >= http.StatusBadRequest:
		_, _ = io.Copy(io.Discard, res.Body)
		return interfaces.ErrNoRoute
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// thin keeps at most limit points, evenly spaced, the first and the last always.
func thin(points []models.LatLng, limit int) []models.LatLng {
	if len(points) <= limit {
		return points
	}
	out := make([]models.LatLng, 0, limit)
	step := float64(len(points)-1) / float64(limit-1)
	for i := 0; i < limit; i++ {
		out = append(out, points[int(float64(i)*step+0.5)])
	}
	return out
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
