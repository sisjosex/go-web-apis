//go:build integration
// +build integration

package geo_test

import (
	"net/http"
	"testing"

	"josex/web/modules/geo/models"
)

func TestEtaFallsBackWhenRoutingIsDown(t *testing.T) {
	helper := setupGeoTest(t)

	body := map[string]any{
		"from":    map[string]float64{"lat": cochabambaLat, "lng": cochabambaLng},
		"to":      map[string]float64{"lat": -17.3700, "lng": -66.1450},
		"costing": "bus",
	}
	w := helper.DoRequest(http.MethodPost, "/geo/eta", body, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("eta: %d %s", w.Code, w.Body.String())
	}
	eta := decode[models.Eta](t, w.Body.Bytes())
	if eta.EstimateSource != models.EstimateFallback {
		t.Fatalf("want a fallback estimate, got %+v", eta)
	}
	// ≈2.9 km straight, ×1.3 ≈ 3.8 km, at 25 km/h ≈ 9 min.
	if eta.DistanceM < 3500 || eta.DistanceM > 4100 || eta.DurationS < 480 || eta.DurationS > 600 {
		t.Fatalf("fallback out of range: %+v", eta)
	}
}

func TestRouteIsUnavailableWhenRoutingIsDown(t *testing.T) {
	helper := setupGeoTest(t)

	body := map[string]any{
		"locations": []map[string]float64{
			{"lat": cochabambaLat, "lng": cochabambaLng},
			{"lat": -17.3700, "lng": -66.1450},
		},
		"costing": "bus",
	}
	w := helper.DoRequest(http.MethodPost, "/geo/route", body, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d %s", w.Code, w.Body.String())
	}
	if got := decode[map[string]any](t, w.Body.Bytes())["error"]; got != "geo.route.unavailable" {
		t.Fatalf("want geo.route.unavailable, got %v", got)
	}
}

func TestRouteRefusesAnUnknownCosting(t *testing.T) {
	helper := setupGeoTest(t)

	body := map[string]any{
		"locations": []map[string]float64{{"lat": -17.39, "lng": -66.15}, {"lat": -17.37, "lng": -66.14}},
		"costing":   "helicopter",
	}
	w := helper.DoRequest(http.MethodPost, "/geo/route", body, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", w.Code, w.Body.String())
	}
}
