//go:build integration
// +build integration

package geo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/geo/repositories"
)

// Central Cochabamba, where the fixture's streets are (INFRA-003 D1).
const (
	cochabambaLat = -17.3935
	cochabambaLng = -66.1570
)

func init() {
	testhelpers.InitTestEnvironment()
	// Nothing listens on port 1: routing is down for the whole package, whatever runs on :8002, so
	// the ETA fallback and the route 503 are what these tests see.
	_ = os.Setenv("GEO_VALHALLA_URL", "http://127.0.0.1:1")
	_ = os.Setenv("GEO_TIMEOUT", "1s")
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
