//go:build integration
// +build integration

package geo_test

import (
	"fmt"
	"net/http"
	"testing"

	"josex/web/modules/geo/models"
)

func TestGeocodeRanksTheNearbyMatchFirst(t *testing.T) {
	helper := setupGeoTest(t)
	importFixture(t, helper)

	// "Aroma" the village is the exact name, but 220 km away; the avenue is here.
	w := helper.DoRequest(http.MethodGet,
		fmt.Sprintf("/geo/geocode?q=aroma&lat=%f&lng=%f", cochabambaLat, cochabambaLng), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("geocode: %d %s", w.Code, w.Body.String())
	}
	res := decode[models.GeocodeResponse](t, w.Body.Bytes())
	if len(res.Results) != 2 {
		t.Fatalf("want Av. Aroma and Aroma, got %+v", res.Results)
	}
	first := res.Results[0]
	if first.Name != "Av. Aroma" || first.Kind != "street" {
		t.Fatalf("first result %+v, want the street Av. Aroma", first)
	}
	if first.City == nil || *first.City != "Cochabamba" {
		t.Fatalf("Av. Aroma should take the nearest city, got %v", first.City)
	}
}

func TestGeocodeFoldsAccentsAndCase(t *testing.T) {
	helper := setupGeoTest(t)
	importFixture(t, helper)

	w := helper.DoRequest(http.MethodGet, "/geo/geocode?q=MERCADO%2025", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("geocode: %d %s", w.Code, w.Body.String())
	}
	res := decode[models.GeocodeResponse](t, w.Body.Bytes())
	if len(res.Results) == 0 || res.Results[0].Name != "Mercado 25 de Mayo" || res.Results[0].Kind != "poi" {
		t.Fatalf("want the market first, got %+v", res.Results)
	}
}

func TestGeocodeRefusesHalfAPoint(t *testing.T) {
	helper := setupGeoTest(t)

	w := helper.DoRequest(http.MethodGet, "/geo/geocode?q=aroma&lat=-17.39", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", w.Code, w.Body.String())
	}
}

func TestReverseReturnsThePlaceAtAPoint(t *testing.T) {
	helper := setupGeoTest(t)
	importFixture(t, helper)

	w := helper.DoRequest(http.MethodGet, "/geo/geocode?q=av%20aroma", nil, nil)
	street := decode[models.GeocodeResponse](t, w.Body.Bytes()).Results[0]

	w = helper.DoRequest(http.MethodGet, fmt.Sprintf("/geo/reverse?lat=%f&lng=%f", street.Lat, street.Lng), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("reverse: %d %s", w.Code, w.Body.String())
	}
	if got := decode[models.Place](t, w.Body.Bytes()); got.Name != "Av. Aroma" {
		t.Fatalf("reverse on Av. Aroma's point returned %+v", got)
	}

	// Nothing of the fixture within 500 m of the middle of the altiplano.
	w = helper.DoRequest(http.MethodGet, "/geo/reverse?lat=-19.5&lng=-67.5", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 far from every place, got %d %s", w.Code, w.Body.String())
	}
}

func TestGeoRoutesNeedAuth(t *testing.T) {
	helper := setupGeoTest(t)
	helper.ClearToken()

	w := helper.DoRequest(http.MethodGet, "/geo/geocode?q=aroma", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without a token, got %d", w.Code)
	}
}
