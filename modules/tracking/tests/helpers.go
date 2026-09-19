// filepath: modules/tracking/tests/helpers.go
//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// Test fixtures - UUIDs that match seed_tracking.sql
const (
	// Companies
	MainCompanyID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	SecondaryCompanyID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	// Vehicles
	TestBusID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	TestVanID = "dddddddd-dddd-dddd-dddd-dddddddddddd"

	// Riders
	TestRiderJohnID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	TestRiderJaneID = "ffffffff-ffff-ffff-ffff-ffffffffffff"

	// Routes
	MorningRouteID   = "11111111-1111-1111-1111-111111111111"
	AfternoonRouteID = "22222222-2222-2222-2222-222222222222"

	// Route Stops
	CentralStationID   = "33333333-3333-3333-3333-333333333333"
	SchoolAStopID      = "44444444-4444-4444-4444-444444444444"
	SchoolBStopID      = "55555555-5555-5555-5555-555555555555"
	DowntownTerminalID = "66666666-6666-6666-6666-666666666666"

	// Assignments
	JohnMorningAssignmentID   = "77777777-7777-7777-7777-777777777777"
	JaneAfternoonAssignmentID = "88888888-8888-8888-8888-888888888888"
)

// ============================================================================
// TEST SETUP HELPER
// ============================================================================

// SetupTrackingTest initializes test environment with auth and tenant context.
// Uses admin@test.local which has the "admin" tenant role and bypasses permission checks.
func SetupTrackingTest(t *testing.T) *testhelpers.ApiTestHelper {
	helper := testhelpers.SetupApiTest(t)
	helper.Login("admin@test.local", "Admin123!")
	helper.SetTenantSlug("test-company")
	return helper
}

// ============================================================================
// COMPANY TEST BUILDERS
// ============================================================================

// ValidCompanyDto returns a valid company DTO with test values
func ValidCompanyDto() models.CreateCompanyDto {
	return models.CreateCompanyDto{
		Name:               "Test Company " + uuid.New().String()[:8],
		Phone:              "+1234567890",
		Address:            "123 Test Street",
		RegistrationNumber: "REG-" + uuid.New().String()[:8],
	}
}

// CompanyDtoWithName returns a company DTO with a specific name
func CompanyDtoWithName(name string) models.CreateCompanyDto {
	dto := ValidCompanyDto()
	dto.Name = name
	return dto
}

// ============================================================================
// VEHICLE TEST BUILDERS
// ============================================================================

// ValidVehicleDto returns a valid vehicle DTO
func ValidVehicleDto() models.CreateVehicleDto {
	companyID := uuid.MustParse(MainCompanyID)
	return models.CreateVehicleDto{
		CompanyID:   companyID,
		PlateNumber: "TST-" + uuid.New().String()[:8],
		VehicleType: "bus",
		Brand:       stringPtr("Mercedes"),
		Model:       stringPtr("Sprinter"),
		Year:        int32Ptr(2023),
		Capacity:    50,
		Status:      "active",
	}
}

// VehicleDtoForCompany returns a vehicle DTO for specific company
func VehicleDtoForCompany(companyID uuid.UUID) models.CreateVehicleDto {
	dto := ValidVehicleDto()
	dto.CompanyID = companyID
	return dto
}

// ============================================================================
// ROUTE TEST BUILDERS
// ============================================================================

// ValidRouteDto returns a valid route DTO
func ValidRouteDto() models.CreateRouteDto {
	companyID := uuid.MustParse(MainCompanyID)
	return models.CreateRouteDto{
		CompanyID:          companyID,
		RouteName:          "Test Route " + uuid.New().String()[:8],
		OriginAddress:      "123 Origin St",
		DestinationAddress: "456 Destination St",
		ScheduleType:       "morning",
		ScheduledStartTime: stringPtr("07:00:00"),
		ScheduledEndTime:   stringPtr("17:00:00"),
	}
}

// RouteDtoForCompany returns a route DTO for specific company
func RouteDtoForCompany(companyID uuid.UUID) models.CreateRouteDto {
	dto := ValidRouteDto()
	dto.CompanyID = companyID
	return dto
}

// ============================================================================
// RIDER TEST BUILDERS
// ============================================================================

// ValidRiderDto returns a valid rider DTO
func ValidRiderDto() models.CreateRiderDto {
	companyID := uuid.MustParse(MainCompanyID)
	email := "rider-" + uuid.New().String()[:8] + "@test.local"
	phone := "+1234567890"
	return models.CreateRiderDto{
		CompanyID: companyID,
		RiderType: "student",
		FirstName: "Test",
		LastName:  "Rider",
		Email:     &email,
		Phone:     &phone,
	}
}

// RiderDtoForCompany returns a rider DTO for specific company
func RiderDtoForCompany(companyID uuid.UUID) models.CreateRiderDto {
	dto := ValidRiderDto()
	dto.CompanyID = companyID
	return dto
}

// ============================================================================
// RESPONSE ASSERTION HELPERS
// ============================================================================

// AssertCompanyResponse validates company response structure
func AssertCompanyResponse(t *testing.T, data map[string]interface{}) {
	assert.NotNil(t, data["id"])
	assert.NotNil(t, data["name"])
	assert.NotNil(t, data["phone"])
	assert.NotNil(t, data["address"])
}

// AssertVehicleResponse validates vehicle response structure
func AssertVehicleResponse(t *testing.T, data map[string]interface{}) {
	assert.NotNil(t, data["id"])
	assert.NotNil(t, data["company_id"])
	assert.NotNil(t, data["plate_number"])
	assert.NotNil(t, data["vehicle_type"])
}

// AssertRouteResponse validates route response structure
func AssertRouteResponse(t *testing.T, data map[string]interface{}) {
	assert.NotNil(t, data["id"])
	assert.NotNil(t, data["company_id"])
	assert.NotNil(t, data["route_name"])
}

// AssertRiderResponse validates rider response structure
func AssertRiderResponse(t *testing.T, data map[string]interface{}) {
	assert.NotNil(t, data["id"])
	assert.NotNil(t, data["company_id"])
	assert.NotNil(t, data["first_name"])
	assert.NotNil(t, data["last_name"])
}

// ============================================================================
// RESPONSE PARSING HELPERS
// ============================================================================

// ExtractID extracts ID from response
func ExtractID(t *testing.T, data map[string]interface{}) string {
	id, ok := data["id"].(string)
	assert.True(t, ok, "Failed to extract ID from response")
	return id
}

// ExtractListCount extracts count from list response
func ExtractListCount(t *testing.T, data []map[string]interface{}) int {
	return len(data)
}

// ParseResponse parses JSON response body
func ParseResponse(t *testing.T, body []byte) map[string]interface{} {
	var response map[string]interface{}
	err := json.Unmarshal(body, &response)
	assert.NoError(t, err, "Failed to parse response JSON")
	return response
}

// ParseListResponse parses JSON list response body
func ParseListResponse(t *testing.T, body []byte) []map[string]interface{} {
	var response []map[string]interface{}
	err := json.Unmarshal(body, &response)
	assert.NoError(t, err, "Failed to parse list response JSON")
	return response
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// stringPtr returns pointer to string
func stringPtr(s string) *string {
	return &s
}

// int32Ptr returns pointer to int32
func int32Ptr(i int32) *int32 {
	return &i
}

// boolPtr returns pointer to bool
func boolPtr(b bool) *bool {
	return &b
}

// uuidPtr returns pointer to UUID
func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}

// ============================================================================
// ROUTE STOP TEST BUILDERS
// ============================================================================

// ValidRouteStopDto returns a valid route stop DTO
func ValidRouteStopDto() models.CreateRouteStopDto {
	routeID := uuid.MustParse(MorningRouteID)
	return models.CreateRouteStopDto{
		RouteID:       routeID,
		StopName:      "Test Stop " + uuid.New().String()[:8],
		StopAddress:   "123 Test Address",
		Latitude:      40.7128,
		Longitude:     -74.0060,
		SequenceOrder: 1,
	}
}

// ============================================================================
// ASSIGNMENT TEST BUILDERS
// ============================================================================

// ValidAssignmentDto returns a valid assignment DTO
func ValidAssignmentDto() models.AssignRiderDto {
	riderID := uuid.MustParse(TestRiderJohnID)
	routeID := uuid.MustParse(MorningRouteID)
	pickupStop := uuid.MustParse(CentralStationID)
	dropoffStop := uuid.MustParse(SchoolAStopID)
	return models.AssignRiderDto{
		RiderID:       riderID,
		RouteID:       routeID,
		PickupStopID:  &pickupStop,
		DropoffStopID: &dropoffStop,
	}
}

// CreateTestRider creates a rider in MainCompanyID and returns its id
func CreateTestRider(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	dto := ValidRiderDto()
	body := map[string]interface{}{
		"company_id": dto.CompanyID,
		"rider_type": dto.RiderType,
		"first_name": dto.FirstName,
		"last_name":  dto.LastName,
	}
	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// CreateTestRouteAssignment creates a route with the given schedule, assigns riderID to it and returns the route id
func CreateTestRouteAssignment(t *testing.T, helper *testhelpers.ApiTestHelper, riderID, startTime, endTime string) string {
	dto := ValidRouteDto()
	body := map[string]interface{}{
		"company_id":           dto.CompanyID,
		"route_name":           dto.RouteName,
		"origin_address":       dto.OriginAddress,
		"destination_address":  dto.DestinationAddress,
		"scheduled_start_time": startTime,
		"scheduled_end_time":   endTime,
	}
	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create route: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	routeID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	w = helper.DoRequest("POST", "/tracking/assignments", map[string]interface{}{"rider_id": riderID, "route_id": routeID}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return routeID
}

// CleanTrackingDatabase cleans only tracking tables while preserving auth and system data
// This is the proper cleanup for tracking-specific tests
func CleanTrackingDatabase(helper *testhelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("tracking")
}
