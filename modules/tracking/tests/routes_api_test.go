//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// ROUTES LIST AND FORM (TRACK-002)
// ============================================================================

// routeBody is a create body on the seeded carrier with a code unique to the call.
func routeBody(code string) map[string]interface{} {
	return map[string]interface{}{
		"company_id":          MainCompanyID,
		"route_name":          "Route " + code,
		"route_code":          code,
		"origin_address":      "Av. Heroínas 100",
		"destination_address": "Av. América 200",
	}
}

// TestListRoutes_PagedSearch - page_size=2&search=<code> → the one route, its company name, the total
func TestListRoutes_PagedSearch(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	code := "R" + uuid.New().String()[:8]
	w := helper.DoRequest("POST", "/tracking/routes", routeBody(code), map[string]string{})
	expectStatus(t, w, http.StatusCreated)

	w = helper.DoRequest("GET", "/tracking/routes?page_size=2&search="+code, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	page := ParseResponse(t, w.Body.Bytes())
	rows := page["routes"].([]interface{})
	if !assert.Len(t, rows, 1) {
		t.FailNow()
	}
	assert.EqualValues(t, 1, page["total_count"])
	assert.EqualValues(t, 2, page["page_size"])
	assert.NotEmpty(t, rows[0].(map[string]interface{})["company_name"])

	// The unfiltered page is cut at page_size and still counts every route.
	w = helper.DoRequest("GET", "/tracking/routes?page_size=2", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	page = ParseResponse(t, w.Body.Bytes())
	assert.Len(t, page["routes"].([]interface{}), 2)
	assert.Greater(t, page["total_count"].(float64), float64(2))
}

// TestCreateRoute_InboundWithDriver - inbound with a vehicle and a driver of the carrier → 201 echoing both
func TestCreateRoute_InboundWithDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	body := routeBody("R" + uuid.New().String()[:8])
	body["direction"] = "inbound"
	body["vehicle_id"] = TestBusID
	body["default_driver_id"] = driverID
	body["timezone"] = "America/La_Paz"

	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	route := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "inbound", route["direction"])
	assert.Equal(t, driverID, route["default_driver_id"])
	assert.Equal(t, TestBusID, route["vehicle_id"])
	assert.Equal(t, "TST-BUS-001", route["license_plate"])
	assert.Equal(t, "Ana Quispe", route["driver_name"])
	assert.Equal(t, "America/La_Paz", route["timezone"])

	// PATCH writes the vehicle and driver as sent: leaving them out clears both.
	w = helper.DoRequest("PATCH", "/tracking/routes/"+route["id"].(string), map[string]interface{}{
		"direction": "outbound",
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	updated := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "outbound", updated["direction"])
	assert.Nil(t, updated["vehicle_id"])
	assert.Nil(t, updated["default_driver_id"])
}

// TestCreateRoute_ForeignDriver - a driver of another carrier → 400 company-mismatch on that field
func TestCreateRoute_ForeignDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// A carrier of its own, so the driver does not show up in other tests' seeded-carrier counts.
	company := ValidCompanyDto()
	w := helper.DoRequest("POST", "/tracking/companies", map[string]interface{}{
		"name":                company.Name,
		"phone":               company.Phone,
		"address":             company.Address,
		"registration_number": company.RegistrationNumber,
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)

	driver := ValidDriverBody()
	driver["company_id"] = ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	w = helper.DoRequest("POST", "/tracking/drivers", driver, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	foreignDriverID := ParseResponse(t, w.Body.Bytes())["id"].(string)

	body := routeBody("R" + uuid.New().String()[:8])
	body["default_driver_id"] = foreignDriverID
	w = helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	resp := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "tracking.route.company-mismatch", resp["error"])
	assert.Equal(t, "default_driver_id", resp["detail"])
}

// TestRiderStatus_RouteWithoutLegacyEndTime - a route created without the legacy hours is still the rider's (D3)
func TestRiderStatus_RouteWithoutLegacyEndTime(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateHomedRider(t, helper)
	routeID := CreateTestRoute(t, helper)
	CreateAssignment(t, helper, AssignmentBody(riderID, routeID, 127))

	w := helper.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Equal(t, routeID, ParseResponse(t, w.Body.Bytes())["route_id"])
}
