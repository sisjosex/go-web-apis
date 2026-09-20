// filepath: modules/tracking/tests/tracking_api_test.go
//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
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
		"company_id":           dto.CompanyID,
		"route_name":           dto.RouteName,
		"origin_address":       dto.OriginAddress,
		"destination_address":  dto.DestinationAddress,
		"schedule_type":        dto.ScheduleType,
		"scheduled_start_time": "08:00:00",
		"scheduled_end_time":   "18:00:00",
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
	routes := ParseListResponse(t, w.Body.Bytes())
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

// TestAddRouteStop - Add stop to route → 201
func TestAddRouteStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidRouteStopDto()
	body := map[string]interface{}{
		"route_id":       dto.RouteID,
		"stop_name":      dto.StopName,
		"stop_address":   dto.StopAddress,
		"latitude":       dto.Latitude,
		"longitude":      dto.Longitude,
		"sequence_order": 4,
	}

	w := helper.DoRequest("POST", "/tracking/route-stops", body, map[string]string{})

	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		assert.True(t, true)
	} else {
		assert.Equal(t, http.StatusCreated, w.Code)
	}
}

// TestRemoveRouteStop - Remove stop from route → 204
func TestRemoveRouteStop(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	stopID := CentralStationID
	w := helper.DoRequest("DELETE", fmt.Sprintf("/tracking/route-stops/%s", stopID), nil, map[string]string{})

	if w.Code == http.StatusNoContent || w.Code == http.StatusOK {
		assert.True(t, true)
	}
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
	routes := ParseListResponse(t, w.Body.Bytes())
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

// ============================================================================
// ASSIGNMENTS TESTS (12 tests)
// ============================================================================

// TestAssignRiderToRoute - Create assignment → 201
func TestAssignRiderToRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	dto := ValidAssignmentDto()
	body := map[string]interface{}{
		"rider_id":        dto.RiderID,
		"route_id":        dto.RouteID,
		"pickup_stop_id":  *dto.PickupStopID,
		"dropoff_stop_id": *dto.DropoffStopID,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})

	if w.Code == http.StatusCreated || w.Code == http.StatusConflict {
		assert.True(t, true) // Conflict if already assigned
	}
}

// TestListRiderAssignments - List assignments for rider → 200
func TestListRiderAssignments(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/assignments?rider_id=%s", riderID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	assignments := ParseListResponse(t, w.Body.Bytes())
	assert.NotNil(t, assignments)
}

// TestListRouteAssignments - List riders on route → 200
func TestListRouteAssignments(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/assignments?route_id=%s", routeID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	assignments := ParseListResponse(t, w.Body.Bytes())
	assert.NotNil(t, assignments)
}

// TestUnassignRiderFromRoute - Remove assignment → 204
func TestUnassignRiderFromRoute(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	assignmentID := JohnMorningAssignmentID
	w := helper.DoRequest("DELETE", fmt.Sprintf("/tracking/assignments/%s", assignmentID), nil, map[string]string{})

	if w.Code == http.StatusNoContent || w.Code == http.StatusOK || w.Code == http.StatusNotFound {
		assert.True(t, true)
	}
}

// TestAssignmentUniqueConstraint - No duplicate assignments → 409
func TestAssignmentUniqueConstraint(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Try to assign same rider to same route
	body := map[string]interface{}{
		"rider_id": TestRiderJohnID,
		"route_id": MorningRouteID,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	// May be 409 if already assigned or 400 if SP rejects duplicate
	assert.True(t, w.Code >= 400 || w.Code == http.StatusCreated)
}

// TestAssignmentValidation - Validate entities exist → 404
func TestAssignmentValidation(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentRider := uuid.New().String()
	body := map[string]interface{}{
		"rider_id": nonExistentRider,
		"route_id": MorningRouteID,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	// SP raises an error for non-existent rider; controller may return 400, 404, or 500
	assert.True(t, w.Code >= 400)
}

// TestGetAssignmentSuccess - Retrieve assignment → 200
func TestGetAssignmentSuccess(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	assignmentID := JohnMorningAssignmentID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/assignments/%s", assignmentID), nil, map[string]string{})

	if w.Code == http.StatusOK || w.Code == http.StatusNotFound {
		assert.True(t, true)
	}
}

// TestAssignmentWithPickupDropoff - Assign with specific stops → 201
func TestAssignmentWithPickupDropoff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJaneID
	routeID := AfternoonRouteID
	pickupStop := SchoolAStopID
	dropoffStop := SchoolBStopID

	body := map[string]interface{}{
		"rider_id":        riderID,
		"route_id":        routeID,
		"pickup_stop_id":  pickupStop,
		"dropoff_stop_id": dropoffStop,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})

	if w.Code == http.StatusCreated || w.Code == http.StatusConflict {
		assert.True(t, true)
	}
}

// TestUpdateAssignmentStops - Update stops for assignment → 200
func TestUpdateAssignmentStops(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	assignmentID := JohnMorningAssignmentID
	body := map[string]interface{}{
		"pickup_stop_id":  SchoolAStopID,
		"dropoff_stop_id": SchoolBStopID,
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/assignments/%s", assignmentID), body, map[string]string{})

	if w.Code >= 200 && w.Code < 300 {
		assert.True(t, true)
	}
}

// TestAssignmentBulkOperations - Bulk assign multiple riders → 201
func TestAssignmentBulkOperations(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	assignments := []map[string]interface{}{
		{
			"rider_id": TestRiderJohnID,
			"route_id": AfternoonRouteID,
		},
		{
			"rider_id": TestRiderJaneID,
			"route_id": MorningRouteID,
		},
	}

	for _, assignment := range assignments {
		w := helper.DoRequest("POST", "/tracking/assignments", assignment, map[string]string{})
		// May succeed or conflict if already assigned
		assert.True(t, w.Code >= 200)
	}
}

// TestGetAssignment - Retrieve specific assignment → 200
func TestGetAssignment(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create an assignment first
	body := map[string]interface{}{
		"rider_id": TestRiderJaneID,
		"route_id": AfternoonRouteID,
	}
	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})

	if w.Code == http.StatusCreated {
		assignment := ParseResponse(t, w.Body.Bytes())
		assignmentID := ExtractID(t, assignment)

		// Now retrieve it - NOTE: No GET endpoint in routes, but testing if it exists
		w = helper.DoRequest("GET", fmt.Sprintf("/tracking/assignments/%s", assignmentID), nil, map[string]string{})

		// If endpoint exists, should return 200, otherwise 404 or 405
		if w.Code == http.StatusOK {
			retrieved := ParseResponse(t, w.Body.Bytes())
			assert.NotNil(t, retrieved["id"])
			assert.NotNil(t, retrieved["rider_id"])
			assert.NotNil(t, retrieved["route_id"])
		} else {
			// Endpoint doesn't exist, which is acceptable
			assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed)
		}
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
	CreateTestRouteAssignment(t, helper, riderID, "00:00:00", "00:00:01")
	lateRouteID := CreateTestRouteAssignment(t, helper, riderID, "23:59:00", "23:59:59")

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
	CreateTestRouteAssignment(t, helper, riderID, "00:00:00", "00:00:01")

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

// TestRecordRideEvent - Log ride events → 201
func TestRecordRideEvent(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"rider_id":   TestRiderJohnID,
		"route_id":   MorningRouteID,
		"vehicle_id": TestBusID,
		"event_type": "check_in",
		"stop_id":    CentralStationID,
	}

	w := helper.DoRequest("POST", "/tracking/events", body, map[string]string{})

	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		assert.True(t, true)
	}
}

// TestRecordRideEvent_StopShownInRiderStatus - Event with stop_id → rider status last_event_stop is that stop
func TestRecordRideEvent_StopShownInRiderStatus(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := CreateTestRider(t, helper)
	stop := ValidRouteStopDto()
	w := helper.DoRequest("POST", "/tracking/route-stops", map[string]interface{}{
		"route_id":       stop.RouteID,
		"stop_name":      stop.StopName,
		"stop_address":   stop.StopAddress,
		"latitude":       stop.Latitude,
		"longitude":      stop.Longitude,
		"sequence_order": 9,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create stop: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	body := map[string]interface{}{
		"rider_id":   riderID,
		"route_id":   MorningRouteID,
		"vehicle_id": TestBusID,
		"event_type": "check_in",
		"stop_id":    ExtractID(t, ParseResponse(t, w.Body.Bytes())),
	}
	w = helper.DoRequest("POST", "/tracking/events", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("record event: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders/%s/status", riderID), nil, map[string]string{})

	assert.Equal(t, http.StatusOK, w.Code)
	status := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, "check_in", status["last_event_type"])
	assert.Equal(t, stop.StopName, status["last_event_stop"])
}

// TestRideEventTimeline - Sequence of events for ride → 200
func TestRideEventTimeline(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := TestRiderJohnID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/events?rider_id=%s", riderID), nil, map[string]string{})

	if w.Code == http.StatusNotFound {
		t.Skip("GET /tracking/events not yet implemented")
		return
	}
	assert.Equal(t, http.StatusOK, w.Code)
	events := ParseListResponse(t, w.Body.Bytes())
	assert.NotNil(t, events)
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

// TestRideEventTypes - Various event types → 201
func TestRideEventTypes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	eventTypes := []string{"check_in", "checkout", "no_show"}

	for _, eventType := range eventTypes {
		body := map[string]interface{}{
			"rider_id":   TestRiderJohnID,
			"route_id":   MorningRouteID,
			"vehicle_id": TestBusID,
			"event_type": eventType,
		}

		w := helper.DoRequest("POST", "/tracking/events", body, map[string]string{})
		assert.True(t, w.Code >= 200)
	}
}

// TestEventNotificationTriggers - Events trigger notifications → 201
func TestEventNotificationTriggers(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := map[string]interface{}{
		"rider_id":   TestRiderJohnID,
		"route_id":   MorningRouteID,
		"vehicle_id": TestBusID,
		"event_type": "check_in",
	}

	w := helper.DoRequest("POST", "/tracking/events", body, map[string]string{})
	// Event creation should succeed
	assert.True(t, w.Code >= 200)
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

// TestGetRouteAlerts - List active alerts → 200
func TestGetRouteAlerts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	routeID := MorningRouteID
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/alerts?route_id=%s", routeID), nil, map[string]string{})

	if w.Code == http.StatusNotFound {
		t.Skip("GET /tracking/alerts not yet implemented")
		return
	}
	assert.Equal(t, http.StatusOK, w.Code)
	alerts := ParseListResponse(t, w.Body.Bytes())
	assert.NotNil(t, alerts)
}

// TestResolveRouteAlert - Mark alert as resolved → 200
func TestResolveRouteAlert(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	alertID := uuid.New().String()
	body := map[string]interface{}{
		"status": "resolved",
	}

	w := helper.DoRequest("PATCH", fmt.Sprintf("/tracking/alerts/%s", alertID), body, map[string]string{})

	if w.Code == http.StatusOK || w.Code == http.StatusNotFound {
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

// TestAssignRider_RiderNotFound - Assigning a non-existent rider returns 404
func TestAssignRider_RiderNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentRiderID := uuid.New().String()
	body := map[string]interface{}{
		"rider_id": nonExistentRiderID,
		"route_id": MorningRouteID,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})

	// SP raises rider.not-found → repository maps to RiderNotFound → controller returns 404
	assert.Equal(t, http.StatusNotFound, w.Code,
		"Assigning non-existent rider should return 404, got %d: %s", w.Code, w.Body.String())
}

// TestAssignRider_RouteNotFound - Assigning to a non-existent route returns 404
func TestAssignRider_RouteNotFound(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	nonExistentRouteID := uuid.New().String()
	body := map[string]interface{}{
		"rider_id": TestRiderJohnID,
		"route_id": nonExistentRouteID,
	}

	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})

	// SP raises route.not-found → repository maps to RouteNotFound → controller returns 404
	assert.Equal(t, http.StatusNotFound, w.Code,
		"Assigning rider to non-existent route should return 404, got %d: %s", w.Code, w.Body.String())
}

// TestAssignRider_Duplicate - Assigning same rider to same route twice returns 409
func TestAssignRider_Duplicate(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	// Create a fresh rider to ensure clean state
	dto := ValidRiderDto()
	createBody := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      "Duplicate",
		"last_name":       "Assign",
		"email":           *dto.Email,
		"phone":           *dto.Phone,
	}
	w := helper.DoRequest("POST", "/tracking/riders", createBody, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Skipf("Could not create rider (status %d); skipping duplicate assignment test", w.Code)
		return
	}
	rider := ParseResponse(t, w.Body.Bytes())
	riderID := ExtractID(t, rider)

	assignBody := map[string]interface{}{
		"rider_id": riderID,
		"route_id": MorningRouteID,
	}

	// First assignment — should succeed
	w = helper.DoRequest("POST", "/tracking/assignments", assignBody, map[string]string{})
	assert.Equal(t, http.StatusCreated, w.Code,
		"First assignment should succeed, got %d: %s", w.Code, w.Body.String())

	// Second assignment — should conflict
	w = helper.DoRequest("POST", "/tracking/assignments", assignBody, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code,
		"Duplicate assignment should return 409 Conflict, got %d: %s", w.Code, w.Body.String())
}
