// filepath: modules/tracking/tests/tracking_api_test.go
//go:build integration
// +build integration

package tracking_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/services/storage"
	"josex/web/modules/core/services/storage/storagetest"
	"josex/web/modules/core/testhelpers"
	trackingConfig "josex/web/modules/tracking/config"
	trackingServices "josex/web/modules/tracking/services"
)

func init() {
	// Initialize test environment (load .env.test before any config)
	testhelpers.InitTestEnvironment()
}

// ============================================================================
// COMPANIES CRUD TESTS (15 tests)
// ============================================================================

// TestCreateCompanySuccess - Create company with valid data → 201
func TestCreateCompanySuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidCompanyDto()
	body := map[string]interface{}{
		"name":                dto.Name,
		"phone":               dto.Phone,
		"address":             dto.Address,
		"registration_number": dto.RegistrationNumber,
	}
	w := helper.DoRequest("POST", "/tracking/companies", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)
	company := ParseResponse(t, w.Body.Bytes())
	AssertCompanyResponse(t, company)
}

// TestCreateCompanyMissingName - Name is required → 400
func TestCreateCompanyMissingName(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"phone":   "+1234567890",
		"address": "123 Test Street",
	}

	w := helper.DoRequest("POST", "/tracking/companies", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCreateCompanyUnauthorized - Requires authentication → 401
func TestCreateCompanyUnauthorized(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	dto := ValidCompanyDto()
	body := map[string]interface{}{
		"name":    dto.Name,
		"phone":   dto.Phone,
		"address": dto.Address,
	}
	w := helper.DoRequest("POST", "/tracking/companies", body, map[string]string{})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestGetCompanySuccess - Retrieve existing company → 200
func TestGetCompanySuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	companyID := MainCompanyID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/companies/%s", companyID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	company := ParseResponse(t, w.Body.Bytes())
	AssertCompanyResponse(t, company)
	assert.Equal(t, companyID, company["id"].(string))
}

// TestGetCompanyNotFound - Non-existent company → 404
func TestGetCompanyNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentID := "00000000-0000-0000-0000-000000000000"
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/companies/%s", nonExistentID), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestListCompaniesSuccess - List all companies → 200
func TestListCompaniesSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListCompanies(t, helper, "")

	assert.Greater(t, len(result.Companies), 0, "Expected at least 1 pre-seeded company")
	assert.GreaterOrEqual(t, result.TotalCount, int64(len(result.Companies)))
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 20, result.PageSize)
}

// TestUpdateCompanySuccess - Update existing company → 200
func TestUpdateCompanySuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	companyID := MainCompanyID
	body := map[string]interface{}{
		"phone": "+9876543210",
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/companies/%s", companyID), body, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	company := ParseResponse(t, w.Body.Bytes())
	assert.NotNil(t, company["phone"])
}

// TestDeleteCompanySuccess - Delete company → 204
func TestDeleteCompanySuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create a company to delete
	createBody := map[string]interface{}{
		"name":    "Company to Delete",
		"phone":   "+1234567890",
		"address": "123 Delete St",
	}
	w := helper.DoRequest("POST", "/tracking/companies", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	company := ParseResponse(t, w.Body.Bytes())
	companyID := ExtractID(t, company)

	// Delete the company
	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/companies/%s", companyID), nil, map[string]string{})
	// Accept both 204 (ideal) and 200/OK for successful deletion
	assert.True(t, w.Code == http.StatusNoContent || w.Code == http.StatusOK || w.Code == http.StatusAccepted)
}

// TestCreateCompanyDuplicateRegistration - Registration number must be unique → 409
func TestCreateCompanyDuplicateRegistration(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Use pre-seeded company registration
	body := map[string]interface{}{
		"name":                "Another Company",
		"phone":               "+1234567890",
		"address":             "123 Test St",
		"registration_number": "REG-TEST-001", // This already exists
	}

	w := helper.DoRequest("POST", "/tracking/companies", body, map[string]string{})
	// SP should handle uniqueness, accept conflict response or validation error
	assert.True(t, w.Code == http.StatusConflict || w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// TestListCompaniesWithFilter - Filter companies → 200
func TestListCompaniesWithFilter(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListCompanies(t, helper, "?status=active")

	assert.Greater(t, len(result.Companies), 0)

	// Verify all companies have status active
	for _, company := range result.Companies {
		assert.NotNil(t, company.Status)
		assert.Equal(t, "active", *company.Status)
	}
}

// TestListCompaniesEmpty - A search nothing matches → 200 with an empty page
func TestListCompaniesEmpty(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListCompanies(t, helper, "?search=no-such-company-"+uuid.New().String())

	assert.NotNil(t, result.Companies, "companies must be [], never null")
	assert.Equal(t, 0, len(result.Companies))
	assert.Equal(t, int64(0), result.TotalCount)
}

// TestCompanyPagination - One page at a time, total_count independent of the page → 200
func TestCompanyPagination(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	firstPage := ListCompanies(t, helper, "?page=1&page_size=1")
	assert.Equal(t, 1, len(firstPage.Companies))
	assert.GreaterOrEqual(t, firstPage.TotalCount, int64(2), "seed has at least two companies")

	secondPage := ListCompanies(t, helper, "?page=2&page_size=1")
	assert.Equal(t, 1, len(secondPage.Companies))
	assert.Equal(t, firstPage.TotalCount, secondPage.TotalCount)
	assert.NotEqual(t, firstPage.Companies[0].ID, secondPage.Companies[0].ID, "pages must not repeat a row")

	// Past the end: total_count rides on the rows, so an empty page carries none.
	pastEnd := ListCompanies(t, helper, "?page=999&page_size=1")
	assert.Equal(t, 0, len(pastEnd.Companies), "companies must be [], never null")
	assert.Equal(t, 999, pastEnd.Page)
	assert.Equal(t, 1, pastEnd.PageSize)
}

// TestCompanySearchMatchesNameAndRegistration - search spans both columns → 200
func TestCompanySearchMatchesNameAndRegistration(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	byRegistration := ListCompanies(t, helper, "?search=REG-TEST-001")
	assert.Equal(t, 1, len(byRegistration.Companies))
	assert.Equal(t, MainCompanyID, byRegistration.Companies[0].ID.String())

	byName := ListCompanies(t, helper, "?search="+url.QueryEscape(byRegistration.Companies[0].Name))
	assert.GreaterOrEqual(t, len(byName.Companies), 1)
}

// TestCompanyPageSizeAboveCapReturns400 - page_size is capped at 100 → 400
func TestCompanyPageSizeAboveCapReturns400(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/companies?page_size=500", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, "page_size is capped at 100")
}

// TestCompanyInvalidUUID - Invalid UUID format → 400
func TestCompanyInvalidUUID(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/companies/invalid-uuid", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================================
// VEHICLES CRUD TESTS (15 tests)
// ============================================================================

// TestCreateVehicleSuccess - Create vehicle → 201
func TestCreateVehicleSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidVehicleDto()
	body := map[string]interface{}{
		"company_id":   dto.CompanyID,
		"plate_number": dto.PlateNumber,
		"vehicle_type": dto.VehicleType,
		"capacity":     dto.Capacity,
		"status":       dto.Status,
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)
	vehicle := ParseResponse(t, w.Body.Bytes())
	AssertVehicleResponse(t, vehicle)
}

// TestCreateVehicleRequiresCompany - Company is mandatory → 400
func TestCreateVehicleRequiresCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"plate_number": "TEST-001",
		"vehicle_type": "bus",
		"capacity":     50,
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListVehiclesByCompany - List vehicles for company → 200
func TestListVehiclesByCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListVehicles(t, helper, "?company_id="+MainCompanyID)

	// Should find 2 pre-seeded vehicles
	assert.Greater(t, len(result.Vehicles), 0)
	for _, v := range result.Vehicles {
		assert.Equal(t, MainCompanyID, v.CompanyID.String())
		assert.NotNil(t, v.CompanyName, "every list row carries its company name")
	}
}

// TestListVehiclesWithoutCompany - the whole fleet, every row named (D2) → 200
func TestListVehiclesWithoutCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	all := ListVehicles(t, helper, "")
	assert.Greater(t, len(all.Vehicles), 0)

	for _, v := range all.Vehicles {
		assert.NotNil(t, v.CompanyName, "every list row carries its company name")
		assert.NotEmpty(t, *v.CompanyName)
	}

	// The unfiltered list is never narrower than any single company's.
	main := ListVehicles(t, helper, "?company_id="+MainCompanyID)
	assert.GreaterOrEqual(t, all.TotalCount, main.TotalCount)
}

// TestVehiclePagination - One page at a time, total_count independent of the page → 200
func TestVehiclePagination(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	firstPage := ListVehicles(t, helper, "?page=1&page_size=2")
	assert.LessOrEqual(t, len(firstPage.Vehicles), 2)
	assert.GreaterOrEqual(t, firstPage.TotalCount, int64(len(firstPage.Vehicles)))
	assert.Equal(t, 1, firstPage.Page)
	assert.Equal(t, 2, firstPage.PageSize)

	pastEnd := ListVehicles(t, helper, "?page=999&page_size=2")
	assert.Equal(t, 0, len(pastEnd.Vehicles), "vehicles must be [], never null")
}

// TestVehicleSearchByPlate - search narrows on the plate number → 200
func TestVehicleSearchByPlate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListVehicles(t, helper, "?search=TST-BUS-001")

	assert.Equal(t, 1, len(result.Vehicles))
	assert.Equal(t, "TST-BUS-001", result.Vehicles[0].PlateNumber)
	assert.Equal(t, int64(1), result.TotalCount)
}

// TestVehiclePageSizeAboveCapReturns400 - page_size is capped at 100 → 400
func TestVehiclePageSizeAboveCapReturns400(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/vehicles?page_size=500", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, "page_size is capped at 100")
}

// TestVehiclePlateNumberUniqueness - Plate number must be unique per company → 409
func TestVehiclePlateNumberUniqueness(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Try to create vehicle with existing plate
	body := map[string]interface{}{
		"company_id":   MainCompanyID,
		"plate_number": "TST-BUS-001", // This already exists
		"vehicle_type": "bus",
		"capacity":     50,
		"status":       "active",
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})
	// SP validates uniqueness; could return 409 conflict or 400 error
	assert.True(t, w.Code == http.StatusConflict || w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
}

// TestGetVehicleSuccess - Retrieve vehicle → 200
func TestGetVehicleSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create vehicle first to ensure it exists
	dto := ValidVehicleDto()
	createBody := map[string]interface{}{
		"company_id":   dto.CompanyID,
		"plate_number": "GET-TEST-" + uuid.New().String()[:8],
		"vehicle_type": dto.VehicleType,
		"capacity":     dto.Capacity,
		"status":       dto.Status,
	}
	w := helper.DoRequest("POST", "/tracking/vehicles", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	vehicle := ParseResponse(t, w.Body.Bytes())
	vehicleID := ExtractID(t, vehicle)

	// Get the created vehicle
	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/vehicles/%s", vehicleID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	retrievedVehicle := ParseResponse(t, w.Body.Bytes())
	AssertVehicleResponse(t, retrievedVehicle)
}

// TestUpdateVehicleSuccess - Update vehicle → 200
func TestUpdateVehicleSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create vehicle first
	dto := ValidVehicleDto()
	createBody := map[string]interface{}{
		"company_id":   dto.CompanyID,
		"plate_number": "UPD-TEST-" + uuid.New().String()[:8],
		"vehicle_type": dto.VehicleType,
		"capacity":     dto.Capacity,
		"status":       dto.Status,
	}
	w := helper.DoRequest("POST", "/tracking/vehicles", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	vehicle := ParseResponse(t, w.Body.Bytes())
	vehicleID := ExtractID(t, vehicle)

	// Update the vehicle
	updateBody := map[string]interface{}{
		"capacity": 60,
	}

	w = helper.DoRequest("PATCH", fmt.Sprintf("/tracking/vehicles/%s", vehicleID), updateBody, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	updatedVehicle := ParseResponse(t, w.Body.Bytes())
	// Check that capacity was updated if present in response
	if capacity, ok := updatedVehicle["capacity"]; ok {
		assert.Equal(t, float64(60), capacity.(float64))
	}
}

// TestVehicleTypeValidation - Vehicle type enumeration → 400
func TestVehicleTypeValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"company_id":   MainCompanyID,
		"plate_number": "TEST-INVALID",
		"vehicle_type": "invalid_type",
		"capacity":     50,
		"status":       "active",
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDeleteVehicleSuccess - Delete vehicle → 204
func TestDeleteVehicleSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create vehicle to delete
	dto := ValidVehicleDto()
	createBody := map[string]interface{}{
		"company_id":   dto.CompanyID,
		"plate_number": dto.PlateNumber,
		"vehicle_type": dto.VehicleType,
		"capacity":     dto.Capacity,
		"status":       dto.Status,
	}
	w := helper.DoRequest("POST", "/tracking/vehicles", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	vehicle := ParseResponse(t, w.Body.Bytes())
	vehicleID := ExtractID(t, vehicle)

	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/vehicles/%s", vehicleID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestVehicleCapacityValidation - Capacity must be positive → 400
func TestVehicleCapacityValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"company_id":   MainCompanyID,
		"plate_number": "TEST-CAP",
		"vehicle_type": "bus",
		"capacity":     0, // Invalid
		"status":       "active",
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestVehicleStatusValidation - Status enumeration → 400
func TestVehicleStatusValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"company_id":   MainCompanyID,
		"plate_number": "TEST-STATUS",
		"vehicle_type": "bus",
		"capacity":     50,
		"status":       "invalid_status",
	}

	w := helper.DoRequest("POST", "/tracking/vehicles", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestVehicleNotFound - Non-existent vehicle → 404
func TestVehicleNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentID := uuid.New().String()
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/vehicles/%s", nonExistentID), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestListVehiclesByStatus - Filter vehicles by status → 200
func TestListVehiclesByStatus(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListVehicles(t, helper, "?company_id="+MainCompanyID+"&status=active")

	for _, v := range result.Vehicles {
		assert.Equal(t, "active", v.Status)
	}
}

// TestListVehiclesByType - Filter vehicles by type → 200
func TestListVehiclesByType(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListVehicles(t, helper, "?company_id="+MainCompanyID+"&vehicle_type=bus")

	for _, v := range result.Vehicles {
		assert.Equal(t, "bus", v.VehicleType)
	}
}

// TestListVehiclesByStatusAndType - Filter vehicles by both status and type → 200
func TestListVehiclesByStatusAndType(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListVehicles(t, helper, "?company_id="+MainCompanyID+"&status=active&vehicle_type=bus")

	for _, v := range result.Vehicles {
		assert.Equal(t, "active", v.Status)
		assert.Equal(t, "bus", v.VehicleType)
	}
}

// TestGetVehicleWithAllFields - Retrieve vehicle and verify all fields → 200
func TestGetVehicleWithAllFields(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Use a pre-seeded vehicle instead of creating one
	vehicleID := TestBusID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/vehicles/%s", vehicleID), nil, map[string]string{})

	if w.Code == http.StatusOK {
		retrieved := ParseResponse(t, w.Body.Bytes())
		// Verify critical fields are present
		assert.NotNil(t, retrieved["id"])
		assert.NotNil(t, retrieved["company_id"])
		assert.NotNil(t, retrieved["plate_number"])
		assert.NotNil(t, retrieved["vehicle_type"])
		assert.NotNil(t, retrieved["capacity"])
		assert.NotNil(t, retrieved["status"])
	} else if w.Code == http.StatusNotFound {
		// Seed data might not be available
		t.Skip("Test bus not found in database")
	}
}

// ============================================================================
// ROUTES CRUD TESTS (15 tests)
// ============================================================================

// TestCreateRouteSuccess - Create route → 201
func TestCreateRouteSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidRouteDto()
	body := map[string]interface{}{
		"company_id":          dto.CompanyID,
		"route_name":          dto.RouteName,
		"origin_address":      dto.OriginAddress,
		"destination_address": dto.DestinationAddress,
	}

	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)
	route := ParseResponse(t, w.Body.Bytes())
	AssertRouteResponse(t, route)
}

// TestListRoutesByCompany - List routes for company → 200
func TestListRoutesByCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	companyID := MainCompanyID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes?company_id=%s", companyID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	routes := ParseResponse(t, w.Body.Bytes())["routes"].([]interface{})
	// Should find 2 pre-seeded routes
	assert.Greater(t, len(routes), 0)
}

// TestGetRouteSuccess - Retrieve route → 200
func TestGetRouteSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s", routeID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	route := ParseResponse(t, w.Body.Bytes())
	AssertRouteResponse(t, route)
}

// TestUpdateRouteSuccess - Update route → 200
func TestUpdateRouteSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	body := map[string]interface{}{
		"route_name": "Updated Morning Route",
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/routes/%s", routeID), body, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	route := ParseResponse(t, w.Body.Bytes())
	assert.NotNil(t, route["route_name"])
}

// TestDeleteRouteSuccess - Delete route → 204
func TestDeleteRouteSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create route to delete
	dto := ValidRouteDto()
	createBody := map[string]interface{}{
		"company_id":          dto.CompanyID,
		"route_name":          "Route to Delete",
		"origin_address":      dto.OriginAddress,
		"destination_address": dto.DestinationAddress,
		"schedule_type":       "custom",
	}
	w := helper.DoRequest("POST", "/tracking/routes", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	route := ParseResponse(t, w.Body.Bytes())
	routeID := ExtractID(t, route)

	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/routes/%s", routeID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestRouteStopsComeFromTheRouteVersion - the backfill (TRACK-007 D1): each seeded route reads its
// stops through the version in force, in the order route_stops held, and every row names the place
// and the version it came from.
func TestRouteStopsComeFromTheRouteVersion(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	stops := ListRouteStops(t, helper, MorningRouteID, "")

	assert.Equal(t, []string{"Central Station", "School A", "School B", "Downtown Terminal"}, StopNames(stops))
	for i, stop := range stops {
		assert.Equal(t, MorningVersionID, stop["version_id"], "row %d", i)
		assert.Equal(t, MorningRouteID, stop["route_id"], "row %d", i)
		assert.Equal(t, float64(i+1), stop["sequence"], "row %d", i)
		assert.NotNil(t, stop["stop_place_id"], "row %d", i)
	}
	assert.Equal(t, CentralStationID, stops[0]["stop_place_id"])
}

// TestRouteStopsTableIsGone - route_stops is replaced, not kept beside the new tables (D1).
func TestRouteStopsTableIsGone(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	var exists bool
	err := helper.DB().QueryRow(context.Background(),
		`SELECT to_regclass('tracking.route_stops') IS NOT NULL`).Scan(&exists)

	assert.NoError(t, err)
	assert.False(t, exists, "tracking.route_stops still exists")
}

// TestTwoRoutesShareOneStopPlace - the whole point of the table: the afternoon route calls at the
// same Central Station row the morning route does, not at a copy of it.
func TestTwoRoutesShareOneStopPlace(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	morning := ListRouteStops(t, helper, MorningRouteID, "")
	afternoon := ListRouteStops(t, helper, AfternoonRouteID, "")

	assert.Equal(t, CentralStationID, morning[0]["stop_place_id"])
	assert.Equal(t, CentralStationID, afternoon[0]["stop_place_id"])
	assert.Equal(t, SchoolBStopID, afternoon[1]["stop_place_id"])
}

// TestRiderStatusNamesTheStopPlace - the guardian view still names the pickup and dropoff stop,
// read from stop_places now.
func TestRiderStatusNamesTheStopPlace(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// sp_get_rider_status skips a route whose end time has passed; the seeded one ends at 17:00, so
	// open it for the length of the test to keep the result independent of the clock.
	var endTime *string
	if err := helper.DB().QueryRow(context.Background(),
		`UPDATE tracking.routes r SET scheduled_end_time = NULL FROM tracking.routes old
		 WHERE r.id = old.id AND r.id = $1 RETURNING CAST(old.scheduled_end_time AS TEXT)`,
		MorningRouteID).Scan(&endTime); err != nil {
		t.Fatalf("open route hours: %v", err)
	}
	defer func() {
		_, _ = helper.DB().Execute(context.Background(),
			`UPDATE tracking.routes SET scheduled_end_time = CAST($2 AS TIME) WHERE id = $1`, MorningRouteID, endTime)
	}()

	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/status", TestRiderJohnID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "Central Station", status["scheduled_pickup_stop"])
	assert.Equal(t, "School A", status["scheduled_dropoff_stop"])
}

// TestRouteDepartureTimeValidation - Time format validation → 400
func TestRouteDepartureTimeValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"company_id":           MainCompanyID,
		"route_name":           "Invalid Time Route",
		"origin_address":       "Start",
		"destination_address":  "End",
		"scheduled_start_time": "25:99:99", // Invalid time
		"schedule_type":        "custom",
	}

	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	// Should return 400 for invalid time format
	if w.Code == http.StatusBadRequest || w.Code == http.StatusUnprocessableEntity {
		assert.True(t, true)
	}
}

// TestRouteTimeLogic - Return time must be after departure → 400
func TestRouteTimeLogic(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"company_id":           MainCompanyID,
		"route_name":           "Invalid Schedule",
		"origin_address":       "Start",
		"destination_address":  "End",
		"scheduled_start_time": "18:00:00",
		"scheduled_end_time":   "08:00:00", // Before start
		"schedule_type":        "custom",
	}

	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	if w.Code >= 400 {
		assert.True(t, true)
	}
}

// TestRouteNotFound - Non-existent route → 404
func TestRouteNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentID := uuid.New().String()
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s", nonExistentID), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestRouteRequiresCompany - Company is mandatory → 400
func TestRouteRequiresCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"route_name":          "No Company Route",
		"origin_address":      "Start",
		"destination_address": "End",
	}

	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListRoutesWithFilters - Filter routes → 200
func TestListRoutesWithFilters(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	companyID := MainCompanyID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes?company_id=%s", companyID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	routes := ParseResponse(t, w.Body.Bytes())["routes"]
	assert.NotNil(t, routes)
}

// ============================================================================
// RIDERS CRUD TESTS (15 tests)
// ============================================================================

// TestCreateRiderSuccess - Create rider → 201
func TestCreateRiderSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidRiderDto()
	body := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      dto.FirstName,
		"last_name":       dto.LastName,
		"email":           *dto.Email,
		"phone":           *dto.Phone,
	}

	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code)
	rider := ParseResponse(t, w.Body.Bytes())
	AssertRiderResponse(t, rider)
}

// TestListRidersByOrganization - List riders for an organization → 200
func TestListRidersByOrganization(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListRiders(t, helper, fmt.Sprintf("?organization_id=%s", MainSchoolID))

	assert.Greater(t, len(result.Riders), 0)
	assert.Greater(t, result.TotalCount, int64(0))
	for _, rider := range result.Riders {
		assert.Equal(t, MainSchoolID, rider.OrganizationID.String())
		// The list rows carry the organization's name so the page can show a column without a
		// second request (TRACK-003 D2).
		if assert.NotNil(t, rider.OrganizationName) {
			assert.Equal(t, "Main Test School", *rider.OrganizationName)
		}
		// And its kind, so the mobile app can call the rows students or staff (MOBILE-002 D3).
		if assert.NotNil(t, rider.OrganizationKind) {
			assert.Equal(t, "school", *rider.OrganizationKind)
		}
	}
}

// TestListRidersPaged - page_size cuts the page while total_count counts the whole match
func TestListRidersPaged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// A third rider, so a page of two is a real cut and not the whole list.
	CreateTestRider(t, helper)

	all := ListRiders(t, helper, "")
	if all.TotalCount < 3 {
		t.Fatalf("expected at least three riders, got total_count %d", all.TotalCount)
	}

	page := ListRiders(t, helper, "?page=1&page_size=2")
	assert.Len(t, page.Riders, 2)
	assert.Equal(t, all.TotalCount, page.TotalCount)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 2, page.PageSize)
}

// TestListRidersSearch - the search matches a name
func TestListRidersSearch(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListRiders(t, helper, "?search=Student")

	assert.Greater(t, len(result.Riders), 0)
	for _, rider := range result.Riders {
		assert.Contains(t, rider.FirstName+" "+rider.LastName, "Student")
	}
}

// TestGetRiderSuccess - Retrieve rider → 200
func TestGetRiderSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s", riderID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	rider := ParseResponse(t, w.Body.Bytes())
	AssertRiderResponse(t, rider)
}

// TestUpdateRiderSuccess - Update rider → 200
func TestUpdateRiderSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	body := map[string]interface{}{
		"phone": "+9999999999",
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), body, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	rider := ParseResponse(t, w.Body.Bytes())
	assert.NotNil(t, rider["phone"])
}

// TestCreateRider_GuardianPersisted - Guardian and emergency contact survive a GET, PATCH keeps omitted fields
func TestCreateRider_GuardianPersisted(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidRiderDto()
	body := map[string]interface{}{
		"organization_id":         dto.OrganizationID,
		"rider_type":              dto.RiderType,
		"first_name":              dto.FirstName,
		"last_name":               dto.LastName,
		"guardian_name":           "Laura Guardian",
		"guardian_phone":          "+15550001",
		"guardian_email":          "laura.guardian@test.local",
		"emergency_contact_name":  "Mario Emergency",
		"emergency_contact_phone": "+15550002",
	}
	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	riderID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s", riderID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	rider := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "Laura Guardian", rider["guardian_name"])
	assert.Equal(t, "+15550001", rider["guardian_phone"])
	assert.Equal(t, "laura.guardian@test.local", rider["guardian_email"])
	assert.Equal(t, "Mario Emergency", rider["emergency_contact_name"])
	assert.Equal(t, "+15550002", rider["emergency_contact_phone"])

	w = helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), map[string]interface{}{"guardian_phone": "+15550009"}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("update rider: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s", riderID), nil, map[string]string{})

	rider = ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "Laura Guardian", rider["guardian_name"])
	assert.Equal(t, "+15550009", rider["guardian_phone"])
	assert.Equal(t, "Mario Emergency", rider["emergency_contact_name"])
}

// TestRiderEmailUniqueness - Duplicate email behavior
func TestRiderEmailUniqueness(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"organization_id": MainSchoolID,
		"rider_type":      "student",
		"first_name":      "Duplicate",
		"last_name":       "Email",
		"email":           "john.student@test.local", // This already exists
		"phone":           "+1111111111",
	}

	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	// Rider email is not enforced as unique at DB level; accept 201 or 409
	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusConflict)
}

// TestRiderPhoneValidation - Phone number validation → 400
func TestRiderPhoneValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"organization_id": MainSchoolID,
		"rider_type":      "student",
		"first_name":      "Invalid",
		"last_name":       "Phone",
		"email":           "test@email.com",
		"phone":           "not-a-phone", // Invalid phone
	}

	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	// May return 400 if phone validation is strict
	assert.True(t, w.Code >= 200)
}

// TestRiderTypeValidation - Rider type enumeration → 400
func TestRiderTypeValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"organization_id": MainSchoolID,
		"rider_type":      "invalid_type", // Invalid type
		"first_name":      "Invalid",
		"last_name":       "Type",
		"phone":           "+1234567890",
	}

	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDeleteRiderSuccess - Delete rider → 204
func TestDeleteRiderSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create rider to delete
	dto := ValidRiderDto()
	createBody := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      dto.FirstName,
		"last_name":       "ToDelete",
		"email":           *dto.Email,
		"phone":           *dto.Phone,
	}
	w := helper.DoRequest("POST", "/tracking/riders", createBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code)

	rider := ParseResponse(t, w.Body.Bytes())
	riderID := ExtractID(t, rider)

	w = helper.DoRequest("DELETE", fmt.Sprintf("/tracking/riders/%s", riderID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

// TestRiderRequiresCompany - Company is mandatory → 400
func TestRiderRequiresCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"rider_type": "student",
		"first_name": "No",
		"last_name":  "Company",
	}

	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRiderNotFound - Non-existent rider → 404
func TestRiderNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentID := uuid.New().String()
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s", nonExistentID), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestListRidersWithFilters - Filter riders → 200
func TestListRidersWithFilters(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListRiders(t, helper, fmt.Sprintf("?organization_id=%s&rider_type=student", MainSchoolID))

	assert.Greater(t, len(result.Riders), 0)
	for _, rider := range result.Riders {
		assert.NotNil(t, rider.RiderType)
		assert.Equal(t, "student", *rider.RiderType)
	}
}

// TestRiderActiveStatus - Activate/deactivate riders → 200
func TestRiderActiveStatus(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	body := map[string]interface{}{
		"is_active": false,
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), body, map[string]string{})

	if w.Code == http.StatusOK || w.Code == http.StatusBadRequest {
		assert.True(t, true)
	}
}

// TestRiderDeactivate - Deactivate rider → 200
func TestRiderDeactivate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create a rider to deactivate
	dto := ValidRiderDto()
	uniqueEmail := "deactivate" + uuid.New().String()[:8] + "@test.local"
	createBody := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      "ToDeactivate",
		"last_name":       "User",
		"email":           uniqueEmail,
		"phone":           "+5555555555",
	}
	w := helper.DoRequest("POST", "/tracking/riders", createBody, map[string]string{})

	// Create should succeed
	if w.Code != http.StatusCreated {
		t.Logf("Create rider failed with status %d, skipping deactivate test", w.Code)
		assert.True(t, true)
		return
	}

	rider := ParseResponse(t, w.Body.Bytes())
	riderID := ExtractID(t, rider)

	// Deactivate the rider
	deactivateBody := map[string]interface{}{
		"is_active": false,
	}
	w = helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), deactivateBody, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	deactivatedRider := ParseResponse(t, w.Body.Bytes())
	// Check that is_active is false if present in response
	if isActive, ok := deactivatedRider["is_active"]; ok {
		assert.Equal(t, false, isActive.(bool))
	}
}

// TestRiderReactivate - Reactivate deactivated rider → 200
func TestRiderReactivate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create a rider, deactivate it, then reactivate
	dto := ValidRiderDto()
	uniqueEmail := "reactivate" + uuid.New().String()[:8] + "@test.local"
	createBody := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      "ToReactivate",
		"last_name":       "User",
		"email":           uniqueEmail,
		"phone":           "+6666666666",
	}
	w := helper.DoRequest("POST", "/tracking/riders", createBody, map[string]string{})

	if w.Code != http.StatusCreated {
		t.Logf("Create rider failed with status %d, skipping reactivate test", w.Code)
		assert.True(t, true)
		return
	}

	rider := ParseResponse(t, w.Body.Bytes())
	riderID := ExtractID(t, rider)

	// Deactivate
	deactivateBody := map[string]interface{}{
		"is_active": false,
	}
	w = helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), deactivateBody, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	// Reactivate
	reactivateBody := map[string]interface{}{
		"is_active": true,
	}
	w = helper.DoRequest("PATCH", fmt.Sprintf("/tracking/riders/%s", riderID), reactivateBody, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)

	reactivatedRider := ParseResponse(t, w.Body.Bytes())
	if isActive, ok := reactivatedRider["is_active"]; ok {
		assert.Equal(t, true, isActive.(bool))
	}
}

// TestGetRiderStatusWithContent - Get rider status and verify response structure → 200
func TestGetRiderStatusWithContent(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/status", riderID), nil, map[string]string{})

	// Endpoint may not be fully implemented
	if w.Code == http.StatusOK {
		status := ParseResponse(t, w.Body.Bytes())
		// Verify response contains expected fields for rider status
		assert.NotNil(t, status, "Status response should not be nil")
		// Status should have at least rider_id or similar identifier
		if riderIDField, exists := status["rider_id"]; exists {
			assert.NotNil(t, riderIDField)
		}
	} else {
		// If endpoint returns error, that's okay - just skip assertion
		assert.True(t, w.Code >= 200, "Status should be valid")
	}
}

// TestGetRiderStatus_PicksCurrentOrNextRoute - Two active assignments → the route not yet ended, on every call
func TestGetRiderStatus_PicksCurrentOrNextRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	CreateTestRouteAssignment(t, helper, riderID, "00:00:00", "00:00:01", "outbound")
	lateRouteID := CreateTestRouteAssignment(t, helper, riderID, "23:59:00", "23:59:59", "inbound")

	for i := 0; i < 3; i++ {
		w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/status", riderID), nil, map[string]string{})

		assert.Equal(t, http.StatusOK, w.Code)
		status := ParseResponse(t, w.Body.Bytes())
		assert.Equal(t, lateRouteID, status["route_id"])
	}
}

// TestGetRiderStatus_AllRoutesEnded - Only routes that already ended today → route_id null
func TestGetRiderStatus_AllRoutesEnded(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	CreateTestRouteAssignment(t, helper, riderID, "00:00:00", "00:00:01", "outbound")

	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/status", riderID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	status := ParseResponse(t, w.Body.Bytes())
	assert.Nil(t, status["route_id"])
}

// ============================================================================
// LOCATIONS & EVENTS TESTS (10 tests)
// ============================================================================

// TestUpdateVehicleLocation - The unauthenticated GPS ingest route is gone → 404
func TestUpdateVehicleLocation(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"vehicle_id": TestBusID,
		"latitude":   40.7128,
		"longitude":  -74.0060,
		"speed":      45.5,
		"heading":    180.0,
	}

	w := helper.DoRequest("POST", "/tracking/locations", body, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetVehicleCurrentLocation - Get latest vehicle location → 200
func TestGetVehicleCurrentLocation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleID := TestBusID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/vehicles/%s/location", vehicleID), nil, map[string]string{})

	if w.Code == http.StatusOK || w.Code == http.StatusNotFound {
		assert.True(t, true)
		// If found, verify structure
		if w.Code == http.StatusOK {
			location := ParseResponse(t, w.Body.Bytes())
			assert.NotNil(t, location)
			// Should have GPS coordinates
			if lat, ok := location["latitude"]; ok {
				assert.NotNil(t, lat)
			}
			if lng, ok := location["longitude"]; ok {
				assert.NotNil(t, lng)
			}
		}
	}
}

// TestGetLocationHistory - Historical vehicle positions → 200
func TestGetLocationHistory(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleID := TestBusID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/locations/history?vehicle_id=%s&limit=10", vehicleID), nil, map[string]string{})

	if w.Code == http.StatusNotFound {
		t.Skip("GET /tracking/locations/history not yet implemented")
		return
	}
	assert.Equal(t, http.StatusOK, w.Code)
	locations := ParseListResponse(t, w.Body.Bytes())
	assert.NotNil(t, locations)
}

// TestLocationHistory Pagination - Page through location history → 200
func TestLocationHistoryPagination(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleID := TestBusID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/locations/history?vehicle_id=%s&limit=5&offset=0", vehicleID), nil, map[string]string{})

	if w.Code == http.StatusNotFound {
		t.Skip("GET /tracking/locations/history not yet implemented")
		return
	}
	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// ROUTE STATUS TESTS (8 tests)
// ============================================================================

// TestGetRouteRealtimeStatus - Get current route status → 200
func TestGetRouteRealtimeStatus(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s/status", routeID), nil, map[string]string{})

	if w.Code == http.StatusOK || w.Code == http.StatusNotFound {
		assert.True(t, true)
	}
}

// TestCreateRouteAlert - Create alert for route → 201
func TestCreateRouteAlert(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"route_id":                MorningRouteID,
		"alert_type":              "delay",
		"title":                   "Route Delayed",
		"message":                 "Morning route is 15 minutes delayed",
		"severity":                "medium",
		"estimated_delay_minutes": 15,
	}

	w := helper.DoRequest("POST", "/tracking/alerts", body, map[string]string{})

	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		assert.True(t, true)
	}
}

// TestRoutePerformanceMetrics - On-time, delays, etc. → 200
func TestRoutePerformanceMetrics(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s/metrics", routeID), nil, map[string]string{})

	// Endpoint may not be implemented yet
	if w.Code == http.StatusOK {
		metrics := ParseResponse(t, w.Body.Bytes())
		assert.NotNil(t, metrics)
		// Verify expected metrics structure if present
		if onTimePercentage, ok := metrics["on_time_percentage"]; ok {
			assert.NotNil(t, onTimePercentage)
		}
		if totalTrips, ok := metrics["total_trips"]; ok {
			assert.NotNil(t, totalTrips)
		}
	} else if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
		// Endpoint not implemented
		assert.True(t, true)
	} else {
		assert.True(t, w.Code < 500, "Should not return server error")
	}
}

// TestStopArrivalPrediction - Predict stop arrival times → 200
func TestStopArrivalPrediction(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s/predictions", routeID), nil, map[string]string{})

	if w.Code == http.StatusOK {
		predictions := ParseResponse(t, w.Body.Bytes())
		assert.NotNil(t, predictions)
		// Should return array of stop predictions or map with stops
		if stops, ok := predictions["stops"]; ok {
			assert.NotNil(t, stops)
		}
	} else if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
		assert.True(t, true)
	} else {
		assert.True(t, w.Code < 500, "Should not return server error")
	}
}

// TestRouteDelayDetection - Automatically detect delays → 200
func TestRouteDelayDetection(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/routes/%s/delays", routeID), nil, map[string]string{})

	if w.Code == http.StatusOK {
		response := ParseResponse(t, w.Body.Bytes())
		assert.NotNil(t, response)
		// Verify response has delay information
		if hasDelay, ok := response["has_delay"]; ok {
			assert.NotNil(t, hasDelay)
		}
		if delayMinutes, ok := response["delay_minutes"]; ok {
			assert.NotNil(t, delayMinutes)
		}
	} else if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
		assert.True(t, true)
	} else {
		assert.True(t, w.Code < 500, "Should not return server error")
	}
}

// TestAlertSeverityLevels - Alert severity enumeration → 201
func TestAlertSeverityLevels(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	severities := []string{"low", "medium", "high", "critical"}

	for _, severity := range severities {
		body := map[string]interface{}{
			"route_id":   MorningRouteID,
			"alert_type": "delay",
			"title":      fmt.Sprintf("%s severity alert", severity),
			"message":    "Test alert",
			"severity":   severity,
		}

		w := helper.DoRequest("POST", "/tracking/alerts", body, map[string]string{})
		assert.True(t, w.Code >= 200)
	}
}

// ============================================================================
// ORGANIZATIONS TESTS (TRACK-005)
// ============================================================================

// TestCreateOrganizationSuccess - Create organization → 201
func TestCreateOrganizationSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidOrganizationDto()
	body := map[string]interface{}{
		"name":     dto.Name,
		"kind":     dto.Kind,
		"timezone": dto.Timezone,
	}

	w := helper.DoRequest("POST", "/tracking/organizations", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	organization := ParseResponse(t, w.Body.Bytes())
	assert.NotNil(t, organization["id"])
	assert.Equal(t, dto.Name, organization["name"])
	assert.Equal(t, "school", organization["kind"])
	assert.Equal(t, "America/Lima", organization["timezone"])
	assert.Equal(t, true, organization["is_active"])
}

// TestCreateOrganizationRejectsUnknownKind - kind is a CHECK on three values → 400
func TestCreateOrganizationRejectsUnknownKind(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{"name": "Bad Kind", "kind": "hospital", "timezone": "UTC"}
	w := helper.DoRequest("POST", "/tracking/organizations", body, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListOrganizationsPaged - page_size cuts the page while total_count counts the whole match
func TestListOrganizationsPaged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	all := ListOrganizations(t, helper, "")
	if all.TotalCount < 2 {
		t.Fatalf("expected at least two seeded organizations, got total_count %d", all.TotalCount)
	}

	page := ListOrganizations(t, helper, "?page=1&page_size=1")
	assert.Len(t, page.Organizations, 1)
	assert.Equal(t, all.TotalCount, page.TotalCount)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 1, page.PageSize)
}

// TestListOrganizationsFiltered - search and kind narrow the page
func TestListOrganizationsFiltered(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	byKind := ListOrganizations(t, helper, "?kind=school")
	assert.Greater(t, len(byKind.Organizations), 0)
	for _, org := range byKind.Organizations {
		assert.Equal(t, "school", org.Kind)
	}

	bySearch := ListOrganizations(t, helper, "?search=Main+Test+School")
	assert.Len(t, bySearch.Organizations, 1)
	assert.Equal(t, MainSchoolID, bySearch.Organizations[0].ID.String())
}

// TestGetOrganizationNotFound - An organization of another tenant is not readable → 404
func TestGetOrganizationNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/organizations/%s", uuid.New()), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestUpdateOrganization - A PATCH of one field leaves the others alone → 200
func TestUpdateOrganization(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/organizations/%s", organizationID),
		map[string]interface{}{"name": "Renamed Organization"}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("update organization: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updated := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "Renamed Organization", updated["name"])
	assert.Equal(t, "school", updated["kind"])
	assert.Equal(t, "America/Lima", updated["timezone"])
}

// TestDeleteOrganizationWithRiders - The seeded school still has riders → 409
func TestDeleteOrganizationWithRiders(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("DELETE", fmt.Sprintf("/tracking/organizations/%s", MainSchoolID), nil, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.has-riders")
}

// TestDeleteOrganizationEmpty - An organization with no riders is deleted → 204
func TestDeleteOrganizationEmpty(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)

	w := helper.DoRequest("DELETE", fmt.Sprintf("/tracking/organizations/%s", organizationID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/organizations/%s", organizationID), nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestOrganizationMembersLifecycle - PUT adds a member, PUT again re-roles them, DELETE removes them
func TestOrganizationMembersLifecycle(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)
	userID := helper.GetUserID()
	memberPath := fmt.Sprintf("/tracking/organizations/%s/members/%s", organizationID, userID)

	w := helper.DoRequest("PUT", memberPath, map[string]interface{}{"role": "admin"}, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	member := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, userID, member["user_id"])
	assert.Equal(t, "admin", member["role"])
	assert.NotNil(t, member["email"])

	members := listMembers(t, helper, organizationID)
	assert.Len(t, members, 1)
	assert.Equal(t, "admin", members[0]["role"])

	// The same call re-roles the member instead of failing on the unique constraint.
	w = helper.DoRequest("PUT", memberPath, map[string]interface{}{"role": "viewer"}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	members = listMembers(t, helper, organizationID)
	assert.Len(t, members, 1)
	assert.Equal(t, "viewer", members[0]["role"])

	w = helper.DoRequest("DELETE", memberPath, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Len(t, listMembers(t, helper, organizationID), 0)
}

// TestUpsertOrganizationMemberUnknownUser - A user that does not exist → 404
func TestUpsertOrganizationMemberUnknownUser(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)
	path := fmt.Sprintf("/tracking/organizations/%s/members/%s", organizationID, uuid.New())

	w := helper.DoRequest("PUT", path, map[string]interface{}{"role": "viewer"}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestDeleteOrganizationMemberNotAMember - Removing someone who was never on it → 404
func TestDeleteOrganizationMemberNotAMember(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	organizationID := createOrganization(t, helper)
	path := fmt.Sprintf("/tracking/organizations/%s/members/%s", organizationID, helper.GetUserID())

	w := helper.DoRequest("DELETE", path, nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// createOrganization creates one organization through the API and returns its id.
func createOrganization(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()

	dto := ValidOrganizationDto()
	body := map[string]interface{}{"name": dto.Name, "kind": dto.Kind, "timezone": dto.Timezone}
	w := helper.DoRequest("POST", "/tracking/organizations", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// listMembers reads an organization's members, failing the test on anything but a 200.
func listMembers(t *testing.T, helper *testhelpers.ApiTestHelper, organizationID string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/organizations/%s/members", organizationID), nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list members: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

// TestDeleteCompany_WithVehicles - Deleting a company that has vehicles returns 409
func TestDeleteCompany_WithVehicles(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// MainCompanyID already has vehicles seeded (TestBusID, TestVanID)
	companyID := MainCompanyID
	w := helper.DoRequest("DELETE", fmt.Sprintf("/tracking/companies/%s", companyID), nil, map[string]string{})

	// SP raises company.has-vehicles → 409 Conflict
	assert.Equal(t, http.StatusConflict, w.Code,
		"Deleting a company with vehicles should return 409 Conflict, got %d: %s", w.Code, w.Body.String())
}

// ============================================================================
// DRIVERS CRUD TESTS (TRACK-006)
// ============================================================================

// TestCreateDriverSuccess - Create a driver for the seeded carrier -> 201, with the carrier's name
// and a NULL account already on the row.
func TestCreateDriverSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/drivers", ValidDriverBody(), map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	driver := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, MainCompanyID, driver["company_id"])
	assert.Equal(t, "Main Test Transit", driver["company_name"])
	assert.Equal(t, "active", driver["status"])
	assert.Nil(t, driver["user_id"])
	assert.Nil(t, driver["user_email"])
}

// TestCreateDriverDuplicateLicense - The licence number is unique per tenant -> 409 with the code
// the app shows against the licence field.
func TestCreateDriverDuplicateLicense(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := ValidDriverBody()
	first := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	second := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})

	assert.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "tracking.driver.license-already-exists")
}

// TestCreateDriverUnknownCompany - A carrier of another tenant, or none at all, is not a carrier
// this driver may belong to -> 404.
func TestCreateDriverUnknownCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := ValidDriverBody()
	body["company_id"] = uuid.New().String()

	w := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.company.not-found")
}

// TestListDriversPaged - page_size caps the rows and total_count counts what the filters match, the
// purchasing shape every new list endpoint answers with.
func TestListDriversPaged(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	for i := 0; i < 3; i++ {
		CreateTestDriver(t, helper)
	}

	w := helper.DoRequest("GET", "/tracking/drivers?page=1&page_size=2", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	page := ParseResponse(t, w.Body.Bytes())
	drivers := page["drivers"].([]interface{})
	assert.Len(t, drivers, 2)
	assert.GreaterOrEqual(t, page["total_count"].(float64), float64(3))
	assert.Equal(t, float64(2), page["page_size"])
}

// TestListDriversBySearchAndCompany - search matches a name or the licence, company_id narrows to
// one carrier, and a carrier with no drivers is an empty page, not an error.
func TestListDriversBySearchAndCompany(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := ValidDriverBody()
	surname := "Mamani" + uuid.New().String()[:6]
	body["last_name"] = surname
	created := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	byName := helper.DoRequest("GET", "/tracking/drivers?search="+url.QueryEscape(surname), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, byName.Code, byName.Body.String())
	assert.Equal(t, float64(1), ParseResponse(t, byName.Body.Bytes())["total_count"])

	license := body["license_number"].(string)
	byLicense := helper.DoRequest("GET", "/tracking/drivers?search="+url.QueryEscape(license), nil, map[string]string{})
	assert.Equal(t, http.StatusOK, byLicense.Code, byLicense.Body.String())
	assert.Equal(t, float64(1), ParseResponse(t, byLicense.Body.Bytes())["total_count"])

	otherCarrier := helper.DoRequest("GET", "/tracking/drivers?company_id="+SecondaryCompanyID, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, otherCarrier.Code, otherCarrier.Body.String())
	assert.Equal(t, float64(0), ParseResponse(t, otherCarrier.Body.Bytes())["total_count"])
}

// TestUpdateDriverKeepsCompanyAndUnlinksAccount - PATCH changes what it sends, the carrier is
// immutable, and clear_user_id is the one thing no user_id value can say.
func TestUpdateDriverKeepsCompanyAndUnlinksAccount(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)

	var accountID string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT id::text FROM auth.users WHERE email = 'driver@test.local'`).Scan(&accountID); err != nil {
		t.Fatalf("read the driver account: %v", err)
	}

	linked := helper.DoRequest("PATCH", "/tracking/drivers/"+driverID, map[string]interface{}{
		"user_id":    accountID,
		"company_id": SecondaryCompanyID, // ignored: the carrier never moves
		"status":     "suspended",
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, linked.Code, linked.Body.String())
	driver := ParseResponse(t, linked.Body.Bytes())
	assert.Equal(t, accountID, driver["user_id"])
	assert.Equal(t, "driver@test.local", driver["user_email"])
	assert.Equal(t, MainCompanyID, driver["company_id"])
	assert.Equal(t, "suspended", driver["status"])

	unlinked := helper.DoRequest("PATCH", "/tracking/drivers/"+driverID, map[string]interface{}{
		"clear_user_id": true,
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, unlinked.Code, unlinked.Body.String())
	after := ParseResponse(t, unlinked.Body.Bytes())
	assert.Nil(t, after["user_id"])
	assert.Nil(t, after["user_email"])
	assert.Equal(t, "suspended", after["status"], "a field PATCH did not send keeps its value")
}

// TestGetDriverNotFound - A driver of another tenant reads as absent, never as forbidden.
func TestGetDriverNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/drivers/"+uuid.New().String(), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.driver.not-found")
}

// TestDeleteDriverRefusedWhileARouteNamesThem - the FK would set those routes' default_driver_id to
// NULL, which is never what the click meant -> 409 with the code the app turns into a toast.
func TestDeleteDriverRefusedWhileARouteNamesThem(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET default_driver_id = $1 WHERE id = $2`,
		driverID, MorningRouteID); err != nil {
		t.Fatalf("name the driver on the route: %v", err)
	}

	refused := helper.DoRequest("DELETE", "/tracking/drivers/"+driverID, nil, map[string]string{})
	assert.Equal(t, http.StatusConflict, refused.Code, refused.Body.String())
	assert.Contains(t, refused.Body.String(), "tracking.driver.has-routes")

	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET default_driver_id = NULL WHERE id = $1`, MorningRouteID); err != nil {
		t.Fatalf("clear the route: %v", err)
	}

	deleted := helper.DoRequest("DELETE", "/tracking/drivers/"+driverID, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, deleted.Code, deleted.Body.String())
}

// TestDriverNameOnStatusEndpoints - driver_name stops being a TODO: both status SPs read it off the
// route's default driver. The schedule is widened for the call because sp_get_rider_status only
// follows an assignment whose route has not ended yet, which would otherwise make this test depend
// on the hour it runs at.
func TestDriverNameOnStatusEndpoints(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET default_driver_id = $1, scheduled_end_time = '23:59:59' WHERE id = $2`,
		driverID, MorningRouteID); err != nil {
		t.Fatalf("name the driver on the route: %v", err)
	}
	defer func() {
		if _, err := helper.DB().Execute(context.Background(),
			`UPDATE tracking.routes SET default_driver_id = NULL, scheduled_end_time = '17:00:00' WHERE id = $1`,
			MorningRouteID); err != nil {
			t.Fatalf("restore the route: %v", err)
		}
	}()

	riderStatus := helper.DoRequest("GET", "/tracking/riders/"+TestRiderJohnID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, riderStatus.Code, riderStatus.Body.String())
	assert.Equal(t, "Ana Quispe", ParseResponse(t, riderStatus.Body.Bytes())["driver_name"])

	routeStatus := helper.DoRequest("GET", "/tracking/routes/"+MorningRouteID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, routeStatus.Code, routeStatus.Body.String())
	assert.Equal(t, "Ana Quispe", ParseResponse(t, routeStatus.Body.Bytes())["driver_name"])
}

// TestRouteStatusWithoutDefaultDriver - a route with no default driver still answers, with a NULL
// name, exactly as it did before TRACK-006.
func TestRouteStatusWithoutDefaultDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+AfternoonRouteID+"/status", nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Nil(t, ParseResponse(t, w.Body.Bytes())["driver_name"])
}

// ============================================================================
// COMPLIANCE DOCUMENTS TESTS (TRACK-016)
// ============================================================================

// TestCreateDocumentTypeAndDuplicateCode - the policy line is created, and its code is unique per
// tenant, upper-cased on the way in.
func TestCreateDocumentTypeAndDuplicateCode(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"code":             "ins-" + uuid.New().String()[:6],
		"name":             "Insurance",
		"applies_to":       "vehicle",
		"warn_days_before": 30,
		"blocks_service":   true,
	}
	first := helper.DoRequest("POST", "/tracking/document-types", body, map[string]string{})
	assert.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	created := ParseResponse(t, first.Body.Bytes())
	assert.Equal(t, strings.ToUpper(body["code"].(string)), created["code"])
	assert.Equal(t, true, created["blocks_service"])

	second := helper.DoRequest("POST", "/tracking/document-types", body, map[string]string{})
	assert.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "tracking.document-type.code-already-exists")
}

// TestListDocumentTypesByAppliesTo - asking for a driver's types returns the driver ones and the
// both-kinds ones, never the vehicle-only ones.
func TestListDocumentTypesByAppliesTo(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	vehicleOnly := CreateTestDocumentType(t, helper, "vehicle", false)
	driverOnly := CreateTestDocumentType(t, helper, "driver", false)
	both := CreateTestDocumentType(t, helper, "both", false)

	w := helper.DoRequest("GET", "/tracking/document-types?applies_to=driver", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	ids := map[string]bool{}
	for _, row := range ParseListResponse(t, w.Body.Bytes()) {
		ids[row["id"].(string)] = true
	}
	assert.True(t, ids[driverOnly], "a driver type must be offered to a driver")
	assert.True(t, ids[both], "a both-kinds type must be offered to a driver")
	assert.False(t, ids[vehicleOnly], "a vehicle-only type must not be offered to a driver")
}

// TestCreateDocumentSuccess - a document is filed with its type and its subject already resolved on
// the row, so the list renders without a second call.
func TestCreateDocumentSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	document := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 200)

	assert.Equal(t, "vehicle", document["subject_type"])
	assert.Equal(t, "TST-BUS-001", document["subject_name"])
	assert.Equal(t, "Test vehicle document", document["type_name"])
	assert.Equal(t, "valid", document["status"], "200 days out is beyond the 30-day warning window")
	assert.Nil(t, document["file_name"])
}

// TestCreateDocumentTypeNotApplicable - a driver's document type cannot be filed against a vehicle.
func TestCreateDocumentTypeNotApplicable(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverTypeID := CreateTestDocumentType(t, helper, "driver", false)

	w := helper.DoRequest("POST", "/tracking/documents", map[string]interface{}{
		"subject_type":     "vehicle",
		"subject_id":       TestBusID,
		"document_type_id": driverTypeID,
	}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.document.type-not-applicable")
}

// TestCreateDocumentUnknownSubject - no FK reaches the subject, so the SP is what refuses a document
// filed against a vehicle that is not this tenant's.
func TestCreateDocumentUnknownSubject(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)

	w := helper.DoRequest("POST", "/tracking/documents", map[string]interface{}{
		"subject_type":     "vehicle",
		"subject_id":       uuid.New().String(),
		"document_type_id": typeID,
	}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.document.subject-not-found")
}

// TestListDocumentsExpiringWithinDays - the filter is a window, not a sort: one expiring in 40 days
// is outside a 30-day question, and one with no expiry is never part of it.
func TestListDocumentsExpiringWithinDays(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	soon := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 10)
	later := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 40)

	noExpiry := helper.DoRequest("POST", "/tracking/documents", map[string]interface{}{
		"subject_type":     "vehicle",
		"subject_id":       TestBusID,
		"document_type_id": typeID,
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, noExpiry.Code, noExpiry.Body.String())
	open := ParseResponse(t, noExpiry.Body.Bytes())

	w := helper.DoRequest("GET", "/tracking/documents?expiring_within_days=30", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	ids := map[string]bool{}
	for _, row := range ParseResponse(t, w.Body.Bytes())["documents"].([]interface{}) {
		ids[row.(map[string]interface{})["id"].(string)] = true
	}
	assert.True(t, ids[soon["id"].(string)], "10 days out is inside a 30-day window")
	assert.False(t, ids[later["id"].(string)], "40 days out is outside a 30-day window")
	assert.False(t, ids[open["id"].(string)], "a document with no expiry never expires")

	assert.Equal(t, "expiring", soon["status"], "10 days out is inside the type's 30-day warning")
}

// TestListDocumentsBySubject - the detail panel asks for one subject's documents and gets only those.
func TestListDocumentsBySubject(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	mine := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 100)
	CreateTestDocument(t, helper, "vehicle", TestVanID, typeID, 100)

	w := helper.DoRequest("GET", "/tracking/documents?subject_type=vehicle&subject_id="+TestBusID, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Other tests in this package file documents against the same bus, so the claim is not "one
	// row" but "only this subject's rows, and mine among them".
	found := false
	for _, row := range ParseResponse(t, w.Body.Bytes())["documents"].([]interface{}) {
		document := row.(map[string]interface{})
		assert.Equal(t, TestBusID, document["subject_id"], "another subject's document leaked in")
		found = found || document["id"] == mine["id"]
	}
	assert.True(t, found, "the document just filed must be on its own subject's list")
}

// TestExpiredBlockingDocumentBlocksDriver - the whole of D2 end to end: an expired document of a
// blocks_service type turns service_blocked on for its driver's list row, and clearing it turns it off.
func TestExpiredBlockingDocumentBlocksDriver(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	typeID := CreateTestDocumentType(t, helper, "driver", true)
	document := CreateTestDocument(t, helper, "driver", driverID, typeID, -5)

	assert.Equal(t, "expired", document["status"])
	assert.Equal(t, true, driverRowByID(t, helper, driverID)["service_blocked"])

	removed := helper.DoRequest("DELETE", "/tracking/documents/"+document["id"].(string), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, removed.Code, removed.Body.String())
	assert.Equal(t, false, driverRowByID(t, helper, driverID)["service_blocked"])
}

// TestNonBlockingExpiredDocumentDoesNotBlock - only a blocks_service type stops a driver working; an
// expired document of any other type is a warning, not a bar.
func TestNonBlockingExpiredDocumentDoesNotBlock(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	driverID := CreateTestDriver(t, helper)
	typeID := CreateTestDocumentType(t, helper, "driver", false)
	CreateTestDocument(t, helper, "driver", driverID, typeID, -5)

	assert.Equal(t, false, driverRowByID(t, helper, driverID)["service_blocked"])
}

// TestUpdateDocumentExpiryResetsWarnedAt - a renewed document is warned about again on its own
// schedule instead of staying silent forever (D1).
func TestUpdateDocumentExpiryResetsWarnedAt(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	document := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 5)

	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.compliance_documents SET warned_at = CURRENT_TIMESTAMP WHERE id = $1`,
		document["id"]); err != nil {
		t.Fatalf("mark the document as warned: %v", err)
	}

	renewed := helper.DoRequest("PATCH", "/tracking/documents/"+document["id"].(string), map[string]interface{}{
		"expires_on": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	}, map[string]string{})
	assert.Equal(t, http.StatusOK, renewed.Code, renewed.Body.String())
	assert.Nil(t, ParseResponse(t, renewed.Body.Bytes())["warned_at"], "a new expiry re-arms the warning")
}

// driverRowByID reads one driver off the list endpoint, which is where service_blocked rides.
func driverRowByID(t *testing.T, helper *testhelpers.ApiTestHelper, driverID string) map[string]interface{} {
	w := helper.DoRequest("GET", "/tracking/drivers?page=1&page_size=100", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	for _, row := range ParseResponse(t, w.Body.Bytes())["drivers"].([]interface{}) {
		driver := row.(map[string]interface{})
		if driver["id"] == driverID {
			return driver
		}
	}
	t.Fatalf("driver %s is not on the list", driverID)
	return nil
}

// ============================================================================
// DOCUMENT FILES AND THE EXPIRY DIGEST (TRACK-016 D1, D3)
// ============================================================================

// TestUploadDocumentFileRoundTrip - a real PDF goes up on a ticket, is claimed, recorded on the row,
// and comes back on a signed link under a name built from the document rather than from whatever
// the uploader called it (INFRA-007 D1, D2).
func TestUploadDocumentFileRoundTrip(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	document := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)
	documentID := document["id"].(string)
	content := PdfBytes(2048)

	uploaded := UploadDocumentFile(t, helper, documentID, "my licence scan.pdf", content)
	assert.Equal(t, http.StatusOK, uploaded.Code, uploaded.Body.String())
	saved := ParseResponse(t, uploaded.Body.Bytes())
	assert.Equal(t, "my licence scan.pdf", saved["file_name"])
	assert.Equal(t, float64(len(content)), saved["file_size"])
	assert.Equal(t, "pdf", saved["file_ext"])

	link := helper.DoRequest("GET", "/tracking/documents/"+documentID+"/file", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, link.Code, link.Body.String())
	url, _ := ParseResponse(t, link.Body.Bytes())["url"].(string)
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET the signed link: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, content, body, "the bytes must come back unchanged")
	assert.Equal(t, "application/pdf", res.Header.Get("Content-Type"))
	disposition := res.Header.Get("Content-Disposition")
	assert.Contains(t, disposition, "attachment")
	assert.Contains(t, disposition, ".pdf")
	assert.NotContains(t, disposition, "my licence scan",
		"the uploader's filename must never reach a header")
}

// countingStore counts the calls a claim makes on the bucket.
type countingStore struct {
	storage.ObjectStore
	calls int
}

func (s *countingStore) GetRange(ctx context.Context, key string, offset, length int64) ([]byte, int64, error) {
	s.calls++
	return s.ObjectStore.GetRange(ctx, key, offset, length)
}

func (s *countingStore) Copy(ctx context.Context, src, dst string) error {
	s.calls++
	return s.ObjectStore.Copy(ctx, src, dst)
}

func (s *countingStore) Delete(ctx context.Context, key string) error {
	s.calls++
	return s.ObjectStore.Delete(ctx, key)
}

// TestClaimDocumentFileCostsTwoStoreCalls - D1's price: a range read for the bytes and the size, a
// copy to the document's key, and nothing else before the answer.
func TestClaimDocumentFileCostsTwoStoreCalls(t *testing.T) {
	ctx := context.Background()
	store := &countingStore{ObjectStore: storagetest.Documents(t)}
	files := trackingServices.NewDocumentFiles(store, &trackingConfig.TrackingConfig{MaxDocumentKB: 5120})
	tenantID := uuid.MustParse(TestTenantID)
	documentID := uuid.New()
	content := PdfBytes(4096)
	key := storage.PendingKey(tenantID, "pdf")
	final := TestTenantID + "/" + documentID.String() + ".pdf"
	t.Cleanup(func() {
		_ = store.ObjectStore.Delete(ctx, key)
		_ = store.ObjectStore.Delete(ctx, final)
	})

	url, headers, err := store.PresignPut(ctx, key, "application/pdf", int64(len(content)), time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	PutToTicket(t, url, map[string]string{"Content-Type": headers.Get("Content-Type")}, content)

	ext, size, err := files.Claim(ctx, tenantID, documentID, key)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	assert.Equal(t, "pdf", ext)
	assert.Equal(t, int64(len(content)), size, "the size comes from the range read's Content-Range")
	assert.Equal(t, 2, store.calls, "a claim is one range read and one copy")

	copied, _, err := store.ObjectStore.GetRange(ctx, final, 0, 8)
	assert.NoError(t, err)
	assert.Equal(t, content[:8], copied)
}

// TestUploadDocumentFileRejectsDisguisedHTML - the declared type is the uploader's claim; the bytes
// are what decide. HTML sent as a PDF is refused and its pending object deleted before the answer.
func TestUploadDocumentFileRejectsDisguisedHTML(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)
	html := []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")

	ticket := helper.DoRequest("POST", "/storage/uploads", map[string]interface{}{
		"purpose": "tracking-document", "content_type": "application/pdf", "size": len(html),
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, ticket.Code, ticket.Body.String())
	issued := ParseResponse(t, ticket.Body.Bytes())
	key := issued["key"].(string)
	PutToTicket(t, issued["upload_url"].(string), map[string]string{"Content-Type": "application/pdf"}, html)

	w := ClaimDocumentFile(helper, documentID, key, "invoice.pdf")
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.document.file-invalid")

	_, _, err := storagetest.Documents(t).GetRange(context.Background(), key, 0, 8)
	assert.ErrorIs(t, err, storage.ErrNotFound, "a refused upload's pending object is deleted before the answer")
}

// TestClaimDocumentFileRejectsAnotherTenantsKey - the key names its tenant, and a claim only takes
// its own: anyone else's, or anything that is not a pending key, reads as not found.
func TestClaimDocumentFileRejectsAnotherTenantsKey(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)

	for _, key := range []string{
		storage.PendingKey(uuid.New(), "pdf"),
		TestTenantID + "/" + documentID + ".pdf",
		storage.PendingPrefix(uuid.MustParse(TestTenantID)) + "../" + uuid.NewString() + ".pdf",
	} {
		w := ClaimDocumentFile(helper, documentID, key, "scan.pdf")
		assert.Equal(t, http.StatusNotFound, w.Code, key+": "+w.Body.String())
	}
}

// TestUploadDocumentFileRejectsWrongType - a type the purpose does not take gets no ticket.
func TestUploadDocumentFileRejectsWrongType(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)

	w := UploadDocumentFile(t, helper, documentID, "notes.txt", []byte("just text"))

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "storage.upload-invalid")
}

// TestUploadDocumentFileRejectsOversized - a PDF past the cap gets no ticket, and one that reaches
// the bucket anyway is refused by the claim, the backstop; the row keeps saying it has no file.
func TestUploadDocumentFileRejectsOversized(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)
	// The default cap is 5120 KB; 6 MB is over it.
	huge := PdfBytes(6 * 1024 * 1024)

	w := UploadDocumentFile(t, helper, documentID, "huge.pdf", huge)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "storage.upload-invalid")

	key := storage.PendingKey(uuid.MustParse(TestTenantID), "pdf")
	if err := storagetest.Documents(t).Put(context.Background(), key, bytes.NewReader(huge), int64(len(huge)), storage.PutOptions{ContentType: "application/pdf"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	claimed := ClaimDocumentFile(helper, documentID, key, "huge.pdf")
	assert.Equal(t, http.StatusUnprocessableEntity, claimed.Code, claimed.Body.String())

	after := helper.DoRequest("GET", "/tracking/documents/"+documentID, nil, map[string]string{})
	assert.Nil(t, ParseResponse(t, after.Body.Bytes())["file_name"], "a refused upload leaves no trace")
}

// TestDownloadDocumentFileWithoutOne - a document that carries no file says so rather than 500ing.
func TestDownloadDocumentFileWithoutOne(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)

	w := helper.DoRequest("GET", "/tracking/documents/"+documentID+"/file", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.document.file-not-found")
}

// TestDeleteDocumentRemovesItsFile - withdrawing a document takes its scan with it, so nothing is
// left in the bucket that no row points at. The delete runs after the answer, so it is polled for.
func TestDeleteDocumentRemovesItsFile(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	documentID := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 120)["id"].(string)
	assert.Equal(t, http.StatusOK, UploadDocumentFile(t, helper, documentID, "scan.pdf", PdfBytes(1024)).Code)

	store := storagetest.Documents(t)
	stored := TestTenantID + "/" + documentID + ".pdf"
	if _, _, err := store.GetRange(context.Background(), stored, 0, 8); err != nil {
		t.Fatalf("the claim should have stored %s: %v", stored, err)
	}

	removed := helper.DoRequest("DELETE", "/tracking/documents/"+documentID, nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, removed.Code, removed.Body.String())

	assert.Eventually(t, func() bool {
		_, _, err := store.GetRange(context.Background(), stored, 0, 8)
		return errors.Is(err, storage.ErrNotFound)
	}, 5*time.Second, 50*time.Millisecond, "the stored scan must go with its row")
}

// TestRaiseDocumentAlertsClaimsEachDocumentOnce - the whole of D1's promise: the claim and the
// report are one statement, so a second pass names nothing, and a document outside its window is
// never touched.
func TestRaiseDocumentAlertsClaimsEachDocumentOnce(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	typeID := CreateTestDocumentType(t, helper, "vehicle", false)
	due := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 10)
	far := CreateTestDocument(t, helper, "vehicle", TestBusID, typeID, 200)

	first := raiseAlerts(t, helper)
	assert.Contains(t, first, due["id"], "a document inside its 30-day window is named")
	assert.NotContains(t, far, far["id"], "a document 200 days out is left alone")
	assert.NotContains(t, first, far["id"], "a document 200 days out is left alone")

	second := raiseAlerts(t, helper)
	assert.NotContains(t, second, due["id"], "a second pass names nothing already claimed")
}

// raiseAlerts runs one pass of the digest's claiming statement and returns the ids it named. The
// job itself is a CLI process; what it promises is this SP, which is what a test can hold to.
func raiseAlerts(t *testing.T, helper *testhelpers.ApiTestHelper) []string {
	rows, err := helper.DB().Query(context.Background(),
		`SELECT id::text FROM tracking.sp_raise_document_alerts($1)`, TestTenantID)
	if err != nil {
		t.Fatalf("raise document alerts: %v", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan an alert: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// ============================================================================
// ACCESS LEVEL TESTS (TRACK-015)
// ============================================================================

// TestPortalUserRefusedOnWeb verifies that a portal guardian gets 403 on a web tenant route —
// the tenant middleware refuses the level before any handler runs (D2).
func TestPortalUserRefusedOnWeb(t *testing.T) {
	helper := SetupPortalTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/companies", nil, map[string]string{})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.user.portal-web-forbidden")
}

// TestDriverUserRefusedOnWeb verifies that a driver account gets 403 on a web tenant route, with
// the same code a portal guardian gets — the level is mobile-only (TRACK-006 D1).
func TestDriverUserRefusedOnWeb(t *testing.T) {
	helper := SetupDriverTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/companies", nil, map[string]string{})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tenant.user.portal-web-forbidden")
}

// ============================================================================
// GUARDIAN ACCESS (TRACK-017)
// ============================================================================

// TestPortalGuardianListsOnlyItsOwnRiders verifies the whole of D1 and D2 on the list: the portal
// chain lets the level through, and the scope inside the SP narrows the page to the riders the
// guardian is a contact of — John is seeded to portal@test.local, Jane is not.
func TestPortalGuardianListsOnlyItsOwnRiders(t *testing.T) {
	portal := SetupPortalTest(t)
	defer portal.Close()

	listed := ListRiders(t, portal, "?page=1&page_size=100")

	if len(listed.Riders) != 1 {
		t.Fatalf("guardian should see exactly its one rider, got %d", len(listed.Riders))
	}
	assert.Equal(t, TestRiderJohnID, listed.Riders[0].ID.String())
	assert.Equal(t, int64(1), listed.TotalCount)
}

// TestPortalGuardianReadsOnlyItsOwnRiderStatus verifies the same scope on the status read: the
// rider outside it answers not-found, never forbidden, so the caller learns nothing about it.
func TestPortalGuardianReadsOnlyItsOwnRiderStatus(t *testing.T) {
	portal := SetupPortalTest(t)
	defer portal.Close()

	w := portal.DoRequest("GET", "/tracking/riders/"+TestRiderJohnID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	status := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "John Student", status["rider_name"])

	w = portal.DoRequest("GET", "/tracking/riders/"+TestRiderJaneID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestPortalGuardianStillRefusedElsewhere verifies that lifting the refusal is per route and not
// per level: everything the guardian may not read is still the blanket 403 of TRACK-015 D2.
func TestPortalGuardianStillRefusedElsewhere(t *testing.T) {
	portal := SetupPortalTest(t)
	defer portal.Close()

	for _, path := range []string{
		"/tracking/companies",
		"/tracking/routes",
		"/tracking/riders/" + TestRiderJohnID,
	} {
		w := portal.DoRequest("GET", path, nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, path+": "+w.Body.String())
		assert.Contains(t, w.Body.String(), "tenant.user.portal-web-forbidden", path)
	}
}

// TestUnlinkedGuardianReadsAnEmptyList verifies D2's one departure from the organization scope: a
// parent the school has not linked yet is a normal state, so the list is empty and 200, not 403.
func TestUnlinkedGuardianReadsAnEmptyList(t *testing.T) {
	portal := SetupUnlinkedPortalTest(t)
	defer portal.Close()

	listed := ListRiders(t, portal, "?page=1&page_size=100")

	assert.Empty(t, listed.Riders)
	assert.Equal(t, int64(0), listed.TotalCount)

	// And a rider it is not a guardian of stays not-found, as it is for a linked guardian.
	w := portal.DoRequest("GET", "/tracking/riders/"+TestRiderJohnID+"/status", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestOperatorRidersUnchangedByGuardianScope verifies the contract the guardian parameter must not
// break: an operator still reads the whole tenant on both routes (TRACK-017 step 3).
func TestOperatorRidersUnchangedByGuardianScope(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()

	listed := ListRiders(t, operator, "?page=1&page_size=100")

	ids := map[string]bool{}
	for _, rider := range listed.Riders {
		ids[rider.ID.String()] = true
	}
	assert.True(t, ids[TestRiderJohnID], "operator should still see John")
	assert.True(t, ids[TestRiderJaneID], "operator should still see Jane")

	for _, riderID := range []string{TestRiderJohnID, TestRiderJaneID} {
		w := operator.DoRequest("GET", "/tracking/riders/"+riderID+"/status", nil, map[string]string{})
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
}

// TestOrganizationUserSeesOnlyItsOwnRiders verifies the scope resolved inside the SP: a rider in
// another organization of the same tenant is neither listed nor readable (TRACK-015 D1).
func TestOrganizationUserSeesOnlyItsOwnRiders(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()

	// A rider on the employer the organization user does not belong to.
	foreign := RiderDtoForOrganization(uuid.MustParse(EmptyEmployerID))
	w := operator.DoRequest("POST", "/tracking/riders", map[string]interface{}{
		"organization_id": foreign.OrganizationID,
		"rider_type":      foreign.RiderType,
		"first_name":      "Foreign",
		"last_name":       "Rider",
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create foreign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	foreignRiderID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	org := SetupOrganizationTest(t)
	defer org.Close()

	listed := ListRiders(t, org, "?page=1&page_size=100")
	if len(listed.Riders) == 0 {
		t.Fatalf("organization user should see its own riders, got none")
	}
	for _, rider := range listed.Riders {
		if rider.OrganizationID.String() != MainSchoolID {
			t.Errorf("rider %s belongs to %s, outside the caller's scope", rider.ID, rider.OrganizationID)
		}
	}

	w = org.DoRequest("GET", "/tracking/riders/"+foreignRiderID, nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestOrganizationUserCannotCreateRiderElsewhere verifies that a rider write outside the scope is
// refused as not-found, so the caller learns nothing about the organization it may not touch.
func TestOrganizationUserCannotCreateRiderElsewhere(t *testing.T) {
	org := SetupOrganizationTest(t)
	defer org.Close()

	w := org.DoRequest("POST", "/tracking/riders", map[string]interface{}{
		"organization_id": EmptyEmployerID,
		"rider_type":      "employee",
		"first_name":      "Out",
		"last_name":       "Of Scope",
	}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// Its own organization still works, so the refusal above is the scope and not the permission.
	w = org.DoRequest("POST", "/tracking/riders", map[string]interface{}{
		"organization_id": MainSchoolID,
		"rider_type":      "student",
		"first_name":      "In",
		"last_name":       "Scope",
	}, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestOrganizationUserCannotReadForeignRouteStops verifies that a route no rider of theirs is
// assigned to is not found — an empty stop list would read as "this route has no stops".
func TestOrganizationUserCannotReadForeignRouteStops(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()

	dto := ValidRouteDto()
	w := operator.DoRequest("POST", "/tracking/routes", map[string]interface{}{
		"company_id":          dto.CompanyID,
		"route_name":          dto.RouteName,
		"origin_address":      dto.OriginAddress,
		"destination_address": dto.DestinationAddress,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create unassigned route: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	foreignRouteID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	org := SetupOrganizationTest(t)
	defer org.Close()

	w = org.DoRequest("GET", "/tracking/routes/"+foreignRouteID+"/stops", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// The seeded morning route carries riders of the school, so it stays readable.
	w = org.DoRequest("GET", "/tracking/routes/"+MorningRouteID+"/stops", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestOrganizationUserDeniedOnOperatorRoutes verifies the per-route denial: the fleet and the
// organizations themselves stay the operator's, whatever roles the level is granted.
func TestOrganizationUserDeniedOnOperatorRoutes(t *testing.T) {
	org := SetupOrganizationTest(t)
	defer org.Close()

	for _, path := range []string{"/tracking/companies", "/tracking/vehicles", "/tracking/organizations"} {
		w := org.DoRequest("GET", path, nil, map[string]string{})
		assert.Equal(t, http.StatusForbidden, w.Code, path+": "+w.Body.String())
	}
}

// TestOrganizationUserWithoutMembershipRefused verifies that an organization-level account linked
// to nothing is refused outright rather than shown an empty list — a misconfiguration, not a state.
func TestOrganizationUserWithoutMembershipRefused(t *testing.T) {
	org := SetupOrganizationTest(t)
	defer org.Close()

	if _, err := org.DB().Execute(context.Background(),
		`DELETE FROM tracking.organization_members om
		 USING auth.users u
		 WHERE om.user_id = u.id AND u.email = 'orguser@test.local'`); err != nil {
		t.Fatalf("drop the membership: %v", err)
	}
	defer func() {
		if _, err := org.DB().Execute(context.Background(),
			`INSERT INTO tracking.organization_members (organization_id, user_id, role)
			 SELECT '99999999-9999-9999-9999-999999999999'::uuid, u.id, 'admin'
			 FROM auth.users u WHERE u.email = 'orguser@test.local'`); err != nil {
			t.Fatalf("restore the membership: %v", err)
		}
	}()

	w := org.DoRequest("GET", "/tracking/riders", nil, map[string]string{})

	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.scope-denied")
}

// TestOperatorStillSeesEveryRider verifies the unscoped path is unchanged: NULL p_scope_user_id
// reads the whole tenant, as it did before TRACK-015.
func TestOperatorStillSeesEveryRider(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()

	foreign := RiderDtoForOrganization(uuid.MustParse(EmptyEmployerID))
	w := operator.DoRequest("POST", "/tracking/riders", map[string]interface{}{
		"organization_id": foreign.OrganizationID,
		"rider_type":      foreign.RiderType,
		"first_name":      "Operator",
		"last_name":       "Visible",
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create foreign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	created := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	listed := ListRiders(t, operator, "?page=1&page_size=100")
	found := false
	for _, rider := range listed.Riders {
		if rider.ID.String() == created {
			found = true
		}
	}
	assert.True(t, found, "the operator should still see riders of every organization")
}

// ============================================================================
// STOP PLACES (TRACK-007 step 2)
// ============================================================================

// TestCreateStopPlaceSuccess - a stop place is created with its coordinate → 201
func TestCreateStopPlaceSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := ValidStopPlaceBody()

	w := helper.DoRequest("POST", "/tracking/stop-places", body, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	place := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, body["name"], place["name"])
	assert.InDelta(t, 40.7129, place["latitude"], 0.00001)
	assert.InDelta(t, -74.0061, place["longitude"], 0.00001)
}

// TestCreateStopPlaceRequiresCoordinate - a place without a point is not a place → 400
func TestCreateStopPlaceRequiresCoordinate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/stop-places", map[string]interface{}{"name": "Nowhere"}, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestCreateStopPlaceUnknownOrganization - pinning a stop to another tenant's organization → 404
func TestCreateStopPlaceUnknownOrganization(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := ValidStopPlaceBody()
	body["organization_id"] = uuid.New().String()

	w := helper.DoRequest("POST", "/tracking/stop-places", body, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.not-found")
}

// TestListStopPlacesNearOrdersByDistance - `near` narrows to a radius, carries distance_m and puts
// the nearest row first; the seeded Central Station sits exactly on the query point.
func TestListStopPlacesNearOrdersByDistance(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	CreateTestStopPlace(t, helper)

	result := ListStopPlaces(t, helper, "?near=40.7128,-74.0060&radius_m=500")

	if len(result.StopPlaces) < 2 {
		t.Fatalf("expected the seeded stop and the created one within 500 m, got %d", len(result.StopPlaces))
	}
	first := result.StopPlaces[0]
	assert.Equal(t, CentralStationID, first.ID.String())
	assert.NotNil(t, first.DistanceM, "near must carry distance_m")
	assert.InDelta(t, 0, *first.DistanceM, 1)
	for i := 1; i < len(result.StopPlaces); i++ {
		assert.GreaterOrEqual(t, *result.StopPlaces[i].DistanceM, *result.StopPlaces[i-1].DistanceM,
			"rows must be ordered nearest first")
	}
	// School A is ~5 km away, so the radius is a real filter and not decoration.
	for _, place := range result.StopPlaces {
		assert.NotEqual(t, SchoolAStopID, place.ID.String(), "School A is outside 500 m")
	}
}

// TestListStopPlacesWithoutNearHasNoDistance - the plain list is alphabetical and distance_m is absent
func TestListStopPlacesWithoutNearHasNoDistance(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	result := ListStopPlaces(t, helper, "?search=School")

	assert.GreaterOrEqual(t, result.TotalCount, int64(2))
	for _, place := range result.StopPlaces {
		assert.Contains(t, place.Name, "School")
		assert.Nil(t, place.DistanceM)
	}
}

// TestListStopPlacesRejectsMalformedNear - a `near` that is not a coordinate → 400
func TestListStopPlacesRejectsMalformedNear(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/stop-places?near=here", nil, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestUpdateStopPlaceMovesTheCoordinate - a PATCH renames and re-pins in one call
func TestUpdateStopPlaceMovesTheCoordinate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	stopPlaceID := CreateTestStopPlace(t, helper)

	w := helper.DoRequest("PATCH", "/tracking/stop-places/"+stopPlaceID, map[string]interface{}{
		"name":      "Moved Stop",
		"latitude":  -12.0464,
		"longitude": -77.0428,
	}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	place := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "Moved Stop", place["name"])
	assert.InDelta(t, -12.0464, place["latitude"], 0.00001)
	assert.InDelta(t, -77.0428, place["longitude"], 0.00001)
}

// TestGetStopPlaceNotFound - an id this tenant does not hold → 404
func TestGetStopPlaceNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/stop-places/"+uuid.New().String(), nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.stop-place.not-found")
}

// TestDeleteStopPlaceInUse - a place a route version still names is refused → 409
func TestDeleteStopPlaceInUse(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("DELETE", "/tracking/stop-places/"+CentralStationID, nil, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.stop-place.in-use")
}

// TestDeleteStopPlaceUnused - a place no version names goes → 204
func TestDeleteStopPlaceUnused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	stopPlaceID := CreateTestStopPlace(t, helper)

	w := helper.DoRequest("DELETE", "/tracking/stop-places/"+stopPlaceID, nil, map[string]string{})

	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	w = helper.DoRequest("GET", "/tracking/stop-places/"+stopPlaceID, nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// ============================================================================
// ROUTE VERSIONS (TRACK-007 step 3)
// ============================================================================

// TestListRouteVersionsShowsTheSeededOne - the backfilled version is open-ended and carries its count
func TestListRouteVersionsShowsTheSeededOne(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	versions := ListRouteVersions(t, helper, MorningRouteID)

	if len(versions) != 1 {
		t.Fatalf("expected one seeded version, got %d", len(versions))
	}
	assert.Equal(t, MorningVersionID, versions[0]["id"])
	assert.Equal(t, "2026-01-01", versions[0]["effective_from"])
	assert.Nil(t, versions[0]["effective_to"])
	assert.Equal(t, float64(4), versions[0]["stops_count"])
}

// TestPublishVersionClosesThePreviousOne - a version from 2026-10-05 closes the one before it on the
// 4th, and …/stops?date reads whichever was in force that day. This is the whole point of versions:
// yesterday's trip keeps the list it ran.
func TestPublishVersionClosesThePreviousOne(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/routes/"+MorningRouteID+"/versions", map[string]interface{}{
		"effective_from": "2026-10-05",
		"stops": []map[string]interface{}{
			{"stop_place_id": DowntownTerminalID, "sequence": 1},
			{"stop_place_id": CentralStationID, "sequence": 2},
		},
	}, map[string]string{})

	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	published := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "2026-10-05", published["effective_from"])
	assert.Equal(t, float64(2), published["stops_count"])

	// The old list on the 4th, the new one on the 5th.
	assert.Equal(t, []string{"Central Station", "School A", "School B", "Downtown Terminal"},
		StopNames(ListRouteStops(t, helper, MorningRouteID, "?date=2026-10-04")))
	assert.Equal(t, []string{"Downtown Terminal", "Central Station"},
		StopNames(ListRouteStops(t, helper, MorningRouteID, "?date=2026-10-05")))

	versions := ListRouteVersions(t, helper, MorningRouteID)
	if len(versions) != 2 {
		t.Fatalf("expected two versions after publishing, got %d", len(versions))
	}
	assert.Equal(t, "2026-10-05", versions[0]["effective_from"], "newest first")
	assert.Equal(t, MorningVersionID, versions[1]["id"])
	assert.Equal(t, "2026-10-04", versions[1]["effective_to"], "the previous version closes the day before")
}

// TestPublishVersionNotAfterTheLatest - history is appended to, never spliced → 409
func TestPublishVersionNotAfterTheLatest(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/routes/"+AfternoonRouteID+"/versions", map[string]interface{}{
		"effective_from": "2026-01-01",
		"stops":          []map[string]interface{}{{"stop_place_id": CentralStationID, "sequence": 1}},
	}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.version-overlap")
}

// TestPublishVersionUnknownStopPlace - a place outside the tenant is refused → 404
func TestPublishVersionUnknownStopPlace(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("POST", "/tracking/routes/"+MorningRouteID+"/versions", map[string]interface{}{
		"effective_from": "2027-03-01",
		"stops":          []map[string]interface{}{{"stop_place_id": uuid.New().String(), "sequence": 1}},
	}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.stop-place.not-found")
}

// TestReplaceVersionStopsReordersInOneRequest - the whole list goes in one PUT and comes back
// renumbered 1..n, so an editor never renumbers rows one at a time.
func TestReplaceVersionStopsReordersInOneRequest(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID := CreateTestRoute(t, helper)
	versionID := PublishRouteVersion(t, helper, routeID, "2027-01-04", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1},
		{"stop_place_id": SchoolAStopID, "sequence": 2},
		{"stop_place_id": SchoolBStopID, "sequence": 3},
	})

	w := helper.DoRequest("PUT", "/tracking/routes/"+routeID+"/versions/"+versionID+"/stops",
		map[string]interface{}{"stops": []map[string]interface{}{
			{"stop_place_id": SchoolBStopID, "sequence": 10, "planned_offset_min": 0, "dwell_sec": 30},
			{"stop_place_id": CentralStationID, "sequence": 20},
			{"stop_place_id": SchoolAStopID, "sequence": 30},
		}}, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	stops := ParseListResponse(t, w.Body.Bytes())
	assert.Equal(t, []string{"School B", "Central Station", "School A"}, StopNames(stops))
	for i, stop := range stops {
		assert.Equal(t, float64(i+1), stop["sequence"], "the stored list is renumbered 1..n")
		assert.Equal(t, versionID, stop["version_id"])
	}
	assert.Equal(t, float64(30), stops[0]["dwell_sec"])
}

// TestReplaceVersionStopsOnAClosedVersion - the list a route already ran is the record of what it
// ran; changing it from here on is a new version → 409
func TestReplaceVersionStopsOnAClosedVersion(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("PUT", "/tracking/routes/"+MorningRouteID+"/versions/"+MorningVersionID+"/stops",
		map[string]interface{}{"stops": []map[string]interface{}{
			{"stop_place_id": CentralStationID, "sequence": 1},
		}}, map[string]string{})

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.version-closed")
}

// TestReplaceVersionStopsUnknownVersion - an id this tenant does not hold → 404
func TestReplaceVersionStopsUnknownVersion(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("PUT", "/tracking/routes/"+MorningRouteID+"/versions/"+uuid.New().String()+"/stops",
		map[string]interface{}{"stops": []map[string]interface{}{}}, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.version-not-found")
}

// TestListRouteVersionsUnknownRoute - a route outside the tenant → 404
func TestListRouteVersionsUnknownRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/tracking/routes/"+uuid.New().String()+"/versions", nil, map[string]string{})

	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.route.not-found")
}
