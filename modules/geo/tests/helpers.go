//go:build integration
// +build integration

package geo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/geo/models"
	"josex/web/modules/geo/repositories"
	"josex/web/modules/geo/services/routing"
)

// Central Cochabamba, where the fixture's streets are (INFRA-003 D1).
const (
	cochabambaLat = -17.3935
	cochabambaLng = -66.1570
)

// valhallaIsUp switches the package's stand-in routing service between down (503, the default) and up
// (one fixed trip). Down is what the ETA fallback and the route 503 tests see; a test that needs a route
// turns it up with routingUp. The breaker opens after five failures in a row, which no test reaches.
var valhallaIsUp atomic.Bool

// fakeTrip is the trip the stand-in answers: one leg, 3.8 km in 9 minutes.
var fakeTrip = []models.LatLng{{Lat: cochabambaLat, Lng: cochabambaLng}, {Lat: -17.3700, Lng: -66.1450}}

func init() {
	testhelpers.InitTestEnvironment()
	// A stand-in instead of whatever runs on :8002, so these tests never depend on the map data.
	valhalla := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !valhallaIsUp.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		shape := routing.EncodePolyline6(fakeTrip)
		_, _ = w.Write([]byte(`{"trip":{"summary":{"time":540,"length":3.8},"legs":[` +
			`{"shape":"` + shape + `","summary":{"time":540,"length":3.8}}]}}`))
	}))
	_ = os.Setenv("GEO_VALHALLA_URL", valhalla.URL)
	_ = os.Setenv("GEO_TIMEOUT", "1s")
}

// routingUp makes the stand-in answer for the rest of the test.
func routingUp(t *testing.T) {
	t.Helper()
	valhallaIsUp.Store(true)
	t.Cleanup(func() { valhallaIsUp.Store(false) })
}

// setupGeoTest signs in the seeded tenant admin: geo routes need auth and a tenant.
func setupGeoTest(t *testing.T) *testhelpers.ApiTestHelper {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	if _, err := helper.Login("admin@test.local", "Admin123!"); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	helper.SetTenantSlug("test-company")
	return helper
}

// importFixture replaces geo.places with fixtures/places.geojsonl, the way `cli geo import` does.
func importFixture(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	file, err := os.Open("fixtures/places.geojsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = file.Close() }()

	count, err := repositories.NewPlacesRepository(helper.DB()).Import(context.Background(), file)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if count != 5 {
		t.Fatalf("imported %d places, want 5", count)
	}
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return out
}
