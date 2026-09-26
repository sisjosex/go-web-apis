package interfaces

import (
	"context"
	"errors"

	"josex/web/modules/geo/models"
)

// Router is the routing port (D5): Valhalla today, any engine that answers a route tomorrow.
type Router interface {
	// Route returns the trip through locations in order, or ErrNoRoute / ErrRouteUnavailable.
	Route(ctx context.Context, locations []models.LatLng, costing string) (*models.Route, error)
	// Trace snaps a recorded path to the road network (map matching) and answers the matched line,
	// or ErrNoRoute / ErrRouteUnavailable (TRACK-010).
	Trace(ctx context.Context, points []models.LatLng, costing string) (*models.Trace, error)
}

var (
	// ErrNoRoute is the router's answer that nothing joins the locations — not an outage.
	ErrNoRoute = errors.New("geo: no route")
	// ErrRouteUnavailable is the router down, slow, erroring or behind an open breaker.
	ErrRouteUnavailable = errors.New("geo: routing unavailable")
)

// HistoricalDurations is the ETA's second source (D5): observed travel times between two points.
// Nil until TRACK-010 records them.
type HistoricalDurations interface {
	// Duration reports ok=false when nothing has been observed between from and to.
	Duration(ctx context.Context, from, to models.LatLng, costing string) (seconds float64, ok bool, err error)
}
