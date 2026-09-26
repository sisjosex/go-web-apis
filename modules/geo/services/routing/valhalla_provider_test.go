package routing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"
)

var cochabamba = []models.LatLng{{Lat: -17.3935, Lng: -66.1570}, {Lat: -17.3700, Lng: -66.1450}}

func TestBreakerOpensAfterFiveFailures(t *testing.T) {
	var hits atomic.Int32
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer stub.Close()

	router := newValhallaProvider(stub.URL, time.Second)
	for i := 1; i <= 6; i++ {
		_, err := router.Route(context.Background(), cochabamba, "bus")
		if !errors.Is(err, interfaces.ErrRouteUnavailable) {
			t.Fatalf("call %d: want ErrRouteUnavailable, got %v", i, err)
		}
	}
	if got := hits.Load(); got != breakerFailures {
		t.Fatalf("Valhalla reached %d times, want %d: the sixth call should not leave the open breaker", got, breakerFailures)
	}
}

func TestNoRouteDoesNotOpenTheBreaker(t *testing.T) {
	var hits atomic.Int32
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_code":442,"error":"No path could be found for input"}`))
	}))
	defer stub.Close()

	router := newValhallaProvider(stub.URL, time.Second)
	for i := 1; i <= 7; i++ {
		if _, err := router.Route(context.Background(), cochabamba, "bus"); !errors.Is(err, interfaces.ErrNoRoute) {
			t.Fatalf("call %d: want ErrNoRoute, got %v", i, err)
		}
	}
	if got := hits.Load(); got != 7 {
		t.Fatalf("Valhalla reached %d times, want 7", got)
	}
}

func TestRouteJoinsTheLegs(t *testing.T) {
	// Two legs sharing their joint: (1,2)→(3,4) then (3,4)→(5,6), degrees × 1e-6.
	legA := encodePolyline([][2]int64{{1, 2}, {3, 4}})
	legB := encodePolyline([][2]int64{{3, 4}, {5, 6}})
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"trip":{"summary":{"time":600,"length":3.5},"legs":[` +
			`{"shape":"` + legA + `","summary":{"time":200,"length":1.5}},` +
			`{"shape":"` + legB + `","summary":{"time":400,"length":2}}]}}`))
	}))
	defer stub.Close()

	route, err := newValhallaProvider(stub.URL, time.Second).Route(context.Background(), cochabamba, "bus")
	if err != nil {
		t.Fatal(err)
	}
	if route.DurationS != 600 || route.DistanceM != 3500 || len(route.Legs) != 2 || route.Legs[1].DistanceM != 2000 {
		t.Fatalf("unexpected route %+v", route)
	}
	points, err := decodePolyline(route.Polyline6)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][2]int64{{1, 2}, {3, 4}, {5, 6}}; len(points) != 3 || points[0] != want[0] || points[2] != want[2] {
		t.Fatalf("joined shape %v, want %v", points, want)
	}
}
