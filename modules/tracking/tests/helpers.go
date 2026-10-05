// filepath: modules/tracking/tests/helpers.go
//go:build integration
// +build integration

package tracking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// Test fixtures - UUIDs that match seed_tracking.sql
const (
	// The tenant every tracking fixture belongs to (tenancy/seed_test_tenant.sql).
	TestTenantID = "00000000-0000-0000-0000-000000000001"

	// Companies
	MainCompanyID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	SecondaryCompanyID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	// Organizations
	MainSchoolID    = "99999999-9999-9999-9999-999999999999"
	EmptyEmployerID = "a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1"

	// Vehicles
	TestBusID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	TestVanID = "dddddddd-dddd-dddd-dddd-dddddddddddd"

	// Riders
	TestRiderJohnID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	TestRiderJaneID = "ffffffff-ffff-ffff-ffff-ffffffffffff"

	// Routes
	MorningRouteID   = "11111111-1111-1111-1111-111111111111"
	AfternoonRouteID = "22222222-2222-2222-2222-222222222222"

	// Stop places (TRACK-007). Both seeded routes call at Central Station and at School B —
	// the same two rows, which is the point of the table.
	CentralStationID   = "33333333-3333-3333-3333-333333333333"
	SchoolAStopID      = "44444444-4444-4444-4444-444444444444"
	SchoolBStopID      = "55555555-5555-5555-5555-555555555555"
	DowntownTerminalID = "66666666-6666-6666-6666-666666666666"

	// Route versions — one open-ended version per seeded route, in force since 2026-01-01.
	MorningVersionID   = "a0000000-0000-0000-0000-000000000001"
	AfternoonVersionID = "a0000000-0000-0000-0000-000000000002"

	// Route schedules — open-ended weekday schedules in force since 2026-01-01: Monday to Friday
	// (bitmask 31) at the route's own departure time (TRACK-018). The Afternoon Route also runs back
	// at 16:00, so it has an outbound and an inbound trip each weekday (TRACK-008).
	MorningScheduleID         = "b0000000-0000-0000-0000-000000000001"
	AfternoonScheduleID       = "b0000000-0000-0000-0000-000000000002"
	AfternoonReturnScheduleID = "b0000000-0000-0000-0000-000000000003"
	WeekdaysMask              = 31

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

// SetupPortalTest signs in the seeded portal guardian — a mobile-only account (TRACK-015 D2).
func SetupPortalTest(t *testing.T) *testhelpers.ApiTestHelper {
	helper := testhelpers.SetupApiTest(t)
	helper.Login("portal@test.local", "Portal123!")
	helper.SetTenantSlug("test-company")
	return helper
}

// SetupUnlinkedPortalTest signs in the second seeded guardian, the one the school has linked to no
// rider: its scope is empty, which is an empty list and not a refusal (TRACK-017 D2).
func SetupUnlinkedPortalTest(t *testing.T) *testhelpers.ApiTestHelper {
	helper := testhelpers.SetupApiTest(t)
	helper.Login("portal-unlinked@test.local", "Portal123!")
	helper.SetTenantSlug("test-company")
	return helper
}

// SetupDriverTest signs in the seeded driver account — mobile-only, like portal (TRACK-006 D1).
func SetupDriverTest(t *testing.T) *testhelpers.ApiTestHelper {
	helper := testhelpers.SetupApiTest(t)
	helper.Login("driver@test.local", "Driver123!")
	helper.SetTenantSlug("test-company")
	return helper
}

// SetupOrganizationTest signs in the seeded organization user, a member of MainSchoolID and of
// nothing else: every tracking read it makes is scoped to that one organization (TRACK-015 D1).
func SetupOrganizationTest(t *testing.T) *testhelpers.ApiTestHelper {
	helper := testhelpers.SetupApiTest(t)
	helper.Login("orguser@test.local", "OrgUser123!")
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
		// The fixtures' dates are UTC days; the deployment default (TRACK-038 D4) is tested on its own.
		Timezone: stringPtr("UTC"),
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

// ValidRiderDto returns a valid rider DTO on the seeded school
func ValidRiderDto() models.CreateRiderDto {
	organizationID := uuid.MustParse(MainSchoolID)
	email := "rider-" + uuid.New().String()[:8] + "@test.local"
	phone := "+1234567890"
	return models.CreateRiderDto{
		OrganizationID: organizationID,
		RiderType:      "student",
		FirstName:      "Test",
		LastName:       "Rider",
		Email:          &email,
		Phone:          &phone,
	}
}

// RiderDtoForOrganization returns a rider DTO for a specific organization
func RiderDtoForOrganization(organizationID uuid.UUID) models.CreateRiderDto {
	dto := ValidRiderDto()
	dto.OrganizationID = organizationID
	return dto
}

// ValidOrganizationDto returns a valid organization DTO with test values
func ValidOrganizationDto() models.CreateOrganizationDto {
	return models.CreateOrganizationDto{
		Name:     "Test Organization " + uuid.New().String()[:8],
		Kind:     "school",
		Timezone: "America/Lima",
	}
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
	assert.NotNil(t, data["organization_id"])
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

// ListCompanies calls the paged companies endpoint and decodes its envelope.
// query is the raw query string including its leading "?", or "".
func ListCompanies(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListCompaniesResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/companies"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list companies %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListCompaniesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list companies %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

// ListVehicles calls the paged vehicles endpoint and decodes its envelope.
func ListVehicles(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListVehiclesResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/vehicles"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list vehicles %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListVehiclesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list vehicles %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

// ListRiders calls the paged riders endpoint and decodes its envelope.
func ListRiders(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListRidersResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/riders"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list riders %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListRidersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list riders %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

// ListOrganizations calls the paged organizations endpoint and decodes its envelope.
func ListOrganizations(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListOrganizationsResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/organizations"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list organizations %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListOrganizationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list organizations %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
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
// STOP PLACE AND ROUTE VERSION HELPERS (TRACK-007)
// ============================================================================

// ValidStopPlaceBody returns a create-stop-place body a few metres from the seeded Central Station,
// so a `near` search over that point finds it too.
func ValidStopPlaceBody() map[string]interface{} {
	return map[string]interface{}{
		"name":      "Test Stop " + uuid.New().String()[:8],
		"address":   "123 Test Address",
		"latitude":  40.7129,
		"longitude": -74.0061,
	}
}

// CreateTestStopPlace creates a stop place and returns its id.
func CreateTestStopPlace(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()

	w := helper.DoRequest("POST", "/tracking/stop-places", ValidStopPlaceBody(), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create stop place: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// ListStopPlaces calls the paged stop-places endpoint and decodes its envelope.
// query is the raw query string including its leading "?", or "".
func ListStopPlaces(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListStopPlacesResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/stop-places"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list stop places %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListStopPlacesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list stop places %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

// ListRouteStops reads the stops a route runs on a date; query is the raw query string including
// its leading "?", or "".
func ListRouteStops(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, query string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/stops"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list route stops %q returned %d: %s", query, w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// CreateTestRoute creates a route on the seeded carrier and returns its id. A fresh route carries
// no versions, so a test that publishes one is independent of the seed and of every other test.
func CreateTestRoute(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()

	dto := ValidRouteDto()
	w := helper.DoRequest("POST", "/tracking/routes", map[string]interface{}{
		"company_id":          dto.CompanyID,
		"route_name":          dto.RouteName,
		"origin_address":      dto.OriginAddress,
		"destination_address": dto.DestinationAddress,
		"timezone":            *dto.Timezone,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create route: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// ListRouteVersions reads a route's versions, newest first.
func ListRouteVersions(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/versions", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list route versions returned %d: %s", w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// PublishRouteVersion publishes a version on a route and returns its id.
func PublishRouteVersion(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, effectiveFrom string, stops []map[string]interface{}) string {
	t.Helper()

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/versions", map[string]interface{}{
		"effective_from": effectiveFrom,
		"stops":          stops,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("publish route version: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// StopNames is the ordered names of a route-stop list, which is what an ordering assertion is about.
func StopNames(rows []map[string]interface{}) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row["stop_name"].(string))
	}
	return names
}

// ============================================================================
// SCHEDULE AND CALENDAR HELPERS (TRACK-018)
// ============================================================================

// ListRouteSchedules reads a route's schedules, newest validity first.
func ListRouteSchedules(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/schedules", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list route schedules returned %d: %s", w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// CreateRouteSchedule adds a recurrence to a route and returns its id.
func CreateRouteSchedule(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string, body map[string]interface{}) string {
	t.Helper()

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/schedules", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create route schedule: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// WeekdayScheduleBody is the common case: Monday to Friday, open-ended, no calendar.
func WeekdayScheduleBody(startTime, validFrom string) map[string]interface{} {
	return map[string]interface{}{
		"days_of_week": WeekdaysMask,
		"start_time":   startTime,
		"valid_from":   validFrom,
	}
}

// CreateTestCalendar creates a calendar and returns its id.
func CreateTestCalendar(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	t.Helper()

	w := helper.DoRequest("POST", "/tracking/calendars", map[string]interface{}{
		"name": "Calendar " + uuid.New().String()[:8],
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create calendar: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// ListCalendars calls the paged calendars endpoint and decodes its envelope.
// query is the raw query string including its leading "?", or "".
func ListCalendars(t *testing.T, helper *testhelpers.ApiTestHelper, query string) models.ListCalendarsResponse {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/calendars"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list calendars %q returned %d: %s", query, w.Code, w.Body.String())
	}

	var result models.ListCalendarsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("list calendars %q returned invalid JSON: %v — %s", query, err, w.Body.String())
	}
	return result
}

// ReplaceCalendarDates saves a calendar's whole date list and returns what is stored.
func ReplaceCalendarDates(t *testing.T, helper *testhelpers.ApiTestHelper, calendarID string, dates []map[string]interface{}) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("PUT", "/tracking/calendars/"+calendarID+"/dates", dates, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("replace calendar dates: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// NoServiceDates is a run of consecutive no_service days, which is what a shutdown week looks like.
func NoServiceDates(first string, count int) []map[string]interface{} {
	start, err := time.Parse("2006-01-02", first)
	if err != nil {
		panic(err)
	}
	dates := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		dates = append(dates, map[string]interface{}{
			"date":  start.AddDate(0, 0, i).Format("2006-01-02"),
			"kind":  "no_service",
			"label": "Holiday",
		})
	}
	return dates
}

// CreateRouteException records an exception on a route and returns its id.
func CreateRouteException(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string, body map[string]interface{}) string {
	t.Helper()

	w := helper.DoRequest("POST", "/tracking/routes/"+routeID+"/exceptions", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create route exception: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// ListRouteExceptions reads a route's exceptions; query is the raw query string including its
// leading "?", or "".
func ListRouteExceptions(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, query string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/exceptions"+query, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("list route exceptions %q returned %d: %s", query, w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// CancelDay is the commonest exception of all: one day off, for a reason.
func CancelDay(date, reason string) map[string]interface{} {
	return map[string]interface{}{
		"date_from": date,
		"date_to":   date,
		"kind":      "cancel",
		"payload":   map[string]interface{}{},
		"reason":    reason,
	}
}

// PreviewRoute reads the plan a route runs over a window.
func PreviewRoute(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, from, to string) []map[string]interface{} {
	t.Helper()

	w := helper.DoRequest("GET", "/tracking/routes/"+routeID+"/preview?from="+from+"&to="+to, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("preview route %s..%s returned %d: %s", from, to, w.Code, w.Body.String())
	}
	return ParseListResponse(t, w.Body.Bytes())
}

// ServiceDates is the ordered dates a preview produced, which is what "that day is not in the plan"
// is asserted against.
func ServiceDates(rows []map[string]interface{}) []string {
	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row["service_date"].(string))
	}
	return dates
}

// ============================================================================
// DRIVER TEST BUILDERS (TRACK-006)
// ============================================================================

// ValidDriverBody returns a create-driver body for the seeded carrier, with a licence number no
// other test can collide with.
func ValidDriverBody() map[string]interface{} {
	return map[string]interface{}{
		"company_id":     MainCompanyID,
		"first_name":     "Ana",
		"last_name":      "Quispe",
		"phone":          "+51987654321",
		"license_number": "LIC-" + uuid.New().String()[:8],
		"license_class":  "A-IIIb",
	}
}

// CreateTestDriver creates a driver for the seeded carrier and returns its id.
func CreateTestDriver(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	w := helper.DoRequest("POST", "/tracking/drivers", ValidDriverBody(), map[string]string{})
	if w.Code != 201 {
		t.Fatalf("create driver: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())["id"].(string)
}

// LinkDriverAccount links the seeded driver account (driver@test.local) to driverID, and frees it
// again at cleanup: the API has no endpoint that links an account to a driver.
func LinkDriverAccount(t *testing.T, helper *testhelpers.ApiTestHelper, driverID string) {
	t.Helper()
	ctx := context.Background()
	unlink := `UPDATE tracking.drivers SET user_id = NULL WHERE user_id = (SELECT id FROM auth.users WHERE email = 'driver@test.local')`
	if _, err := helper.DB().Execute(ctx, unlink); err != nil {
		t.Fatalf("unlink driver account: %v", err)
	}
	t.Cleanup(func() { _, _ = helper.DB().Execute(ctx, unlink) })
	if _, err := helper.DB().Execute(ctx, `UPDATE tracking.drivers SET user_id = (SELECT id FROM auth.users WHERE email = 'driver@test.local') WHERE id = $1`, driverID); err != nil {
		t.Fatalf("link driver account: %v", err)
	}
}

// TenantMembership reads an account's level in the test tenant and whether it is active ("" when it
// has none): the API answers access grants, never the tenant_users row they write (TRACK-032).
func TenantMembership(t *testing.T, helper *testhelpers.ApiTestHelper, userID string) (string, bool) {
	t.Helper()
	var role string
	var active bool
	err := helper.DB().QueryRow(context.Background(),
		`SELECT role, is_active FROM tenancy.tenant_users WHERE tenant_id = $1 AND user_id = $2`,
		TestTenantID, userID).Scan(&role, &active)
	if err != nil {
		return "", false
	}
	return role, active
}

// UserIDByEmail is the id of the account holding email, "" when there is none.
func UserIDByEmail(t *testing.T, helper *testhelpers.ApiTestHelper, email string) string {
	t.Helper()
	var id string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT id::text FROM auth.users WHERE email = $1`, email).Scan(&id); err != nil {
		return ""
	}
	return id
}

// CountUsersByEmail counts the accounts holding email — one, after any number of grants (TRACK-032).
func CountUsersByEmail(t *testing.T, helper *testhelpers.ApiTestHelper, email string) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM auth.users WHERE email = $1`, email).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

// SetRouteDefaultDriver sets the driver a route's trips are built with, without the route.changed a
// PATCH writes: a test counting the signals of one later change must not see this one.
func SetRouteDefaultDriver(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, driverID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(), `UPDATE tracking.routes SET default_driver_id = $2 WHERE id = $1`, routeID, driverID); err != nil {
		t.Fatalf("route default driver: %v", err)
	}
}

// ============================================================================
// DOCUMENT TEST BUILDERS (TRACK-016)
// ============================================================================

// CreateTestDocumentType adds one line to the tenant's compliance policy and returns its id. The
// code is unique per call, so tests never collide on the per-tenant uniqueness.
func CreateTestDocumentType(t *testing.T, helper *testhelpers.ApiTestHelper, appliesTo string, blocksService bool) string {
	body := map[string]interface{}{
		"code":             "DOC-" + uuid.New().String()[:8],
		"name":             "Test " + appliesTo + " document",
		"applies_to":       appliesTo,
		"warn_days_before": 30,
		"blocks_service":   blocksService,
	}
	w := helper.DoRequest("POST", "/tracking/document-types", body, map[string]string{})
	if w.Code != 201 {
		t.Fatalf("create document type: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())["id"].(string)
}

// CreateTestDocument files a document against one subject and returns the parsed row.
func CreateTestDocument(t *testing.T, helper *testhelpers.ApiTestHelper, subjectType, subjectID, typeID string, expiresInDays int) map[string]interface{} {
	body := map[string]interface{}{
		"subject_type":     subjectType,
		"subject_id":       subjectID,
		"document_type_id": typeID,
		"number":           "N-" + uuid.New().String()[:6],
		"expires_on":       time.Now().AddDate(0, 0, expiresInDays).Format("2006-01-02"),
	}
	w := helper.DoRequest("POST", "/tracking/documents", body, map[string]string{})
	if w.Code != 201 {
		t.Fatalf("create document: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())
}

// UploadDocumentFile runs the app's whole upload (INFRA-007 D1): a ticket declaring the type the
// extension implies and the content's size, the PUT straight to the bucket, then the claim. It returns
// the ticket's recorder when the ticket is refused and the claim's otherwise, so a test can assert
// the status as well as the body.
func UploadDocumentFile(t *testing.T, helper *testhelpers.ApiTestHelper, documentID, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	contentType := map[string]string{".pdf": "application/pdf", ".jpg": "image/jpeg", ".png": "image/png"}[strings.ToLower(filepath.Ext(filename))]
	if contentType == "" {
		contentType = "text/plain"
	}
	ticket := helper.DoRequest("POST", "/storage/uploads", map[string]interface{}{
		"purpose": "tracking-document", "content_type": contentType, "size": len(content),
	}, map[string]string{})
	if ticket.Code != http.StatusCreated {
		return ticket
	}
	var issued struct {
		Key       string            `json:"key"`
		UploadURL string            `json:"upload_url"`
		Headers   map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(ticket.Body.Bytes(), &issued); err != nil {
		t.Fatalf("ticket: %v", err)
	}
	PutToTicket(t, issued.UploadURL, issued.Headers, content)
	return ClaimDocumentFile(helper, documentID, issued.Key, filename)
}

// PutToTicket uploads content to a signed URL the way the browser does, and fails on anything but 200.
func PutToTicket(t *testing.T, uploadURL string, headers map[string]string, content []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("upload: expected 200 from the bucket, got %d", res.StatusCode)
	}
}

// ClaimDocumentFile sends PUT /tracking/documents/:id/file for an uploaded key.
func ClaimDocumentFile(helper *testhelpers.ApiTestHelper, documentID, key, filename string) *httptest.ResponseRecorder {
	return helper.DoRequest("PUT", "/tracking/documents/"+documentID+"/file",
		map[string]interface{}{"key": key, "filename": filename}, map[string]string{})
}

// PdfBytes returns n bytes that sniff as a PDF — a real header followed by padding, so a size test
// and a content test can use the same builder.
func PdfBytes(n int) []byte {
	head := []byte("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n")
	if n <= len(head) {
		return head
	}
	out := make([]byte, n)
	copy(out, head)
	for i := len(head); i < n; i++ {
		out[i] = ' '
	}
	return out
}

// ============================================================================
// ASSIGNMENT TEST BUILDERS
// ============================================================================

// AssignmentBody is an assignment from the first day of the seed's year, open-ended, on the given
// weekdays: the shape every assignment test starts from.
func AssignmentBody(riderID, routeID string, daysOfWeek int) map[string]interface{} {
	return map[string]interface{}{
		"rider_id":     riderID,
		"route_id":     routeID,
		"days_of_week": daysOfWeek,
		"valid_from":   "2026-01-01",
	}
}

// seedRouteAssignment writes an assignment straight to the table — past the API's overlap check — and
// answers its id; validUntil "" is open-ended.
func seedRouteAssignment(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, riderID string, daysOfWeek int, validFrom, validUntil string) string {
	t.Helper()
	var id string
	err := helper.DB().QueryRow(context.Background(), `
		INSERT INTO tracking.rider_route_assignments (rider_id, route_id, days_of_week, valid_from, valid_until)
		VALUES ($1, $2, $3, $4::DATE, NULLIF($5, '')::DATE) RETURNING id`,
		riderID, routeID, daysOfWeek, validFrom, validUntil).Scan(&id)
	if err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	return id
}

// alertNotices answers the feed rows one alert wrote, payload per rider and recipient email.
func alertNotices(t *testing.T, helper *testhelpers.ApiTestHelper, alertID string) map[string]map[string]map[string]interface{} {
	t.Helper()
	rows, err := helper.DB().Query(t.Context(), `
		SELECT n.rider_id, u.email, n.payload FROM tracking.notifications n JOIN auth.users u ON u.id = n.user_id
		WHERE n.type = 'delay' AND n.payload->>'alert_id' = $1`, alertID)
	if err != nil {
		t.Fatalf("read alert notices: %v", err)
	}
	defer rows.Close()
	byRider := map[string]map[string]map[string]interface{}{}
	for rows.Next() {
		var rider, email string
		var payload map[string]interface{}
		if err := rows.Scan(&rider, &email, &payload); err != nil {
			t.Fatalf("scan alert notice: %v", err)
		}
		if byRider[rider] == nil {
			byRider[rider] = map[string]map[string]interface{}{}
		}
		byRider[rider][email] = payload
	}
	return byRider
}

// notifyAlertRaised runs what the alert.changed worker runs for a raise and answers how many rows it
// gave to push.
func notifyAlertRaised(t *testing.T, helper *testhelpers.ApiTestHelper, alertID string) int {
	t.Helper()
	var pushed int
	if err := helper.DB().QueryRow(t.Context(), `
		SELECT count(*) FROM tracking.sp_notify_event($1, 'alert.changed', jsonb_build_object('type', 'raised', 'alert_id', $2::TEXT), NULL)`,
		TestTenantID, alertID).Scan(&pushed); err != nil {
		t.Fatalf("notify alert: %v", err)
	}
	return pushed
}

// addGuardians makes each seeded account a guardian contact of riderID, and on cleanup removes the
// rider's contacts and notices.
func addGuardians(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string, emails ...string) {
	t.Helper()
	execSQL(t, helper, `INSERT INTO tracking.rider_contacts (rider_id, relation, name, email, user_id, is_primary)
		SELECT $1, 'guardian', u.email, u.email, u.id, FALSE FROM auth.users u WHERE u.email = ANY($2::TEXT[])`, riderID, emails)
	t.Cleanup(func() {
		execSQL(t, helper, `DELETE FROM tracking.rider_contacts WHERE rider_id = $1`, riderID)
		execSQL(t, helper, `DELETE FROM tracking.notifications WHERE rider_id = $1`, riderID)
	})
}

// raiseTripAlert writes an alert on one trip, as the driver's incident does, and answers its id.
func raiseTripAlert(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, tripID, alertType, title string) string {
	t.Helper()
	var id string
	if err := helper.DB().QueryRow(t.Context(), `
		INSERT INTO tracking.route_alerts (route_id, trip_id, alert_type, title) VALUES ($1, $2, $3, $4) RETURNING id`,
		routeID, tripID, alertType, title).Scan(&id); err != nil {
		t.Fatalf("trip alert: %v", err)
	}
	return id
}

// CreateAssignment posts an assignment and returns the response body's assignment.
func CreateAssignment(t *testing.T, helper *testhelpers.ApiTestHelper, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())["assignment"].(map[string]interface{})
}

// CreateTestRider creates a rider in MainSchoolID without a home pin, as an import row with only an
// address leaves one (TRACK-043 D2), and returns its id. Assigning one needs explicit stops.
func CreateTestRider(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	return createRider(t, helper, nil)
}

// CreateHomedRider creates a rider in MainSchoolID with a home pin out of town, so an assignment with no
// stop can place them (TRACK-044 D3), and returns its id. Assigned to a seeded route it adds a stop that
// later tests see; give it a route of its own.
func CreateHomedRider(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	return createRider(t, helper, map[string]interface{}{"home_latitude": -17.35, "home_longitude": -66.2})
}

func createRider(t *testing.T, helper *testhelpers.ApiTestHelper, extra map[string]interface{}) string {
	t.Helper()
	dto := ValidRiderDto()
	body := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      dto.FirstName,
		"last_name":       dto.LastName,
	}
	for k, v := range extra {
		body[k] = v
	}
	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
}

// CreateTestRouteAssignment creates a route with the given schedule and direction, assigns riderID
// to it every day and returns the route id. Two routes of one rider must differ in direction to be
// in force on the same day (TRACK-009 D4); routes are created outbound, so inbound is set directly.
func CreateTestRouteAssignment(t *testing.T, helper *testhelpers.ApiTestHelper, riderID, startTime, endTime, direction string) string {
	t.Helper()
	dto := ValidRouteDto()
	body := map[string]interface{}{
		"company_id":          dto.CompanyID,
		"route_name":          dto.RouteName,
		"origin_address":      dto.OriginAddress,
		"destination_address": dto.DestinationAddress,
		"direction":           direction,
	}
	w := helper.DoRequest("POST", "/tracking/routes", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create route: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	routeID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	// The legacy hours still pick the rider's route in sp_get_rider_status, but no endpoint writes
	// them any more (TRACK-002 D3).
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET scheduled_start_time = $2, scheduled_end_time = $3 WHERE id = $1`,
		routeID, startTime, endTime); err != nil {
		t.Fatalf("set route hours: %v", err)
	}

	CreateAssignment(t, helper, AssignmentBody(riderID, routeID, EveryDayMask))
	return routeID
}

// SetRouteDirection writes a route's direction straight to the table.
func SetRouteDirection(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, direction string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET direction = $2 WHERE id = $1`, routeID, direction); err != nil {
		t.Fatalf("set route direction: %v", err)
	}
}

// RegisterTestPhone gives userID a phone that receives pushes at token — through the SP the device
// endpoint calls — and takes it away when the test ends.
func RegisterTestPhone(t *testing.T, helper *testhelpers.ApiTestHelper, userID, token string) {
	t.Helper()
	ctx := context.Background()
	if _, err := helper.DB().Execute(ctx, `SELECT auth.sp_upsert_push_device($1, 'test-phone', 'android', $2)`, userID, token); err != nil {
		t.Fatalf("register phone: %v", err)
	}
	t.Cleanup(func() {
		if _, err := helper.DB().Execute(ctx, `DELETE FROM auth.push_devices WHERE device_id = 'test-phone'`); err != nil {
			t.Errorf("forget phone: %v", err)
		}
	})
}

// LinkPortalGuardian makes the seeded guardian (portal@test.local) a guardian of riderID too, and
// unlinks them when the test ends — the seeded guardian has exactly one rider everywhere else.
func LinkPortalGuardian(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := helper.DB().Execute(ctx, `INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
		SELECT $1, 'guardian', 'Portal Guardian', '+3333333333', u.email, u.id, FALSE FROM auth.users u WHERE u.email = 'portal@test.local'`, riderID); err != nil {
		t.Fatalf("link guardian: %v", err)
	}
	t.Cleanup(func() {
		if _, err := helper.DB().Execute(ctx, `DELETE FROM tracking.rider_contacts WHERE rider_id = $1`, riderID); err != nil {
			t.Errorf("unlink guardian: %v", err)
		}
	})
}

// CleanTrackingDatabase cleans only tracking tables while preserving auth and system data
// This is the proper cleanup for tracking-specific tests
func CleanTrackingDatabase(helper *testhelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("tracking")
}

// BackfillRowsFor counts the route.changed outbox rows the paths backfill wrote for a route — dateless
// ones (TRACK-028). No endpoint reads the outbox.
func BackfillRowsFor(t *testing.T, helper *testhelpers.ApiTestHelper, routeID string) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM tracking.outbox WHERE topic = 'route.changed' AND payload->>'route_id' = $1 AND payload->>'date_from' IS NULL`,
		routeID).Scan(&n); err != nil {
		t.Fatalf("count backfill rows: %v", err)
	}
	return n
}

// TripVersionID answers the route version a trip was materialised from, whose line a test stores
// (MOBILE-015). No endpoint names it.
func TripVersionID(t *testing.T, helper *testhelpers.ApiTestHelper, tripID string) string {
	t.Helper()
	var versionID string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT route_version_id::text FROM tracking.trips WHERE id = $1`, tripID).Scan(&versionID); err != nil {
		t.Fatalf("trip version: %v", err)
	}
	return versionID
}

// ArriveStopByGPS marks a stop arrived the way sp_ingest_positions does from the positions, with no
// driver op behind it, and answers the arrived_at it stored (MOBILE-015 A1).
func ArriveStopByGPS(t *testing.T, helper *testhelpers.ApiTestHelper, stopID uuid.UUID) time.Time {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`SELECT 1 FROM tracking.sp_trip_stop_transition($1, $2, 'arrive', NULL)`, TestTenantID, stopID); err != nil {
		t.Fatalf("arrive stop: %v", err)
	}
	return StopArrivedAt(t, helper, stopID)
}

// StopArrivedAt answers a stop's stored arrived_at; the driver's API rounds it for display.
func StopArrivedAt(t *testing.T, helper *testhelpers.ApiTestHelper, stopID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT arrived_at FROM tracking.trip_stops WHERE id = $1`, stopID).Scan(&at); err != nil {
		t.Fatalf("read arrived_at: %v", err)
	}
	return at
}

// TripEventCount counts a trip's events of one type. No endpoint lists the trip's events.
func TripEventCount(t *testing.T, helper *testhelpers.ApiTestHelper, tripID uuid.UUID, eventType string) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM tracking.trip_events WHERE trip_id = $1 AND type = $2`, tripID, eventType).Scan(&n); err != nil {
		t.Fatalf("count trip events: %v", err)
	}
	return n
}

// RiderDropoffStatuses answers the statuses of one rider's drop-offs on a trip.
func RiderDropoffStatuses(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, subjectID uuid.UUID) []string {
	t.Helper()
	rows, err := helper.DB().Query(context.Background(), `
		SELECT k.status FROM tracking.trip_stop_tasks k
		JOIN tracking.trip_stops s ON s.id = k.trip_stop_id
		WHERE s.trip_id = $1 AND k.kind = 'dropoff' AND k.subject_id = $2 ORDER BY 1`, tripID, subjectID)
	if err != nil {
		t.Fatalf("read dropoffs: %v", err)
	}
	defer rows.Close()
	statuses := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan dropoff: %v", err)
		}
		statuses = append(statuses, s)
	}
	return statuses
}

// RiderTasks answers one rider's tasks on a trip, sequence:kind:status per task.
func RiderTasks(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, riderID string) []string {
	t.Helper()
	rows, err := helper.DB().Query(context.Background(), `
		SELECT s.sequence || ':' || k.kind || ':' || k.status FROM tracking.trip_stop_tasks k
		JOIN tracking.trip_stops s ON s.id = k.trip_stop_id
		WHERE s.trip_id = $1 AND k.subject_id = $2 ORDER BY 1`, tripID, riderID)
	if err != nil {
		t.Fatalf("read rider tasks: %v", err)
	}
	defer rows.Close()
	tasks := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan rider task: %v", err)
		}
		tasks = append(tasks, s)
	}
	return tasks
}

// InsertOutboxRow writes one outbox row as an SP would, answering its id and the tenant the trigger
// gave it (INFRA-009); err is the trigger's refusal when the payload names neither tenant nor route.
// No endpoint writes a bare row.
func InsertOutboxRow(helper *testhelpers.ApiTestHelper, topic string, payload map[string]interface{}) (int64, string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	var id int64
	var tenantID string
	err = helper.DB().QueryRow(context.Background(),
		`INSERT INTO tracking.outbox (topic, payload) VALUES ($1, $2) RETURNING id, tenant_id`, topic, raw).Scan(&id, &tenantID)
	return id, tenantID, err
}

// InsertOutboxRowIn writes one outbox row inside a transaction the test owns — the rolled-back case.
func InsertOutboxRowIn(tx pgx.Tx, topic string, payload map[string]interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(context.Background(), `INSERT INTO tracking.outbox (topic, payload) VALUES ($1, $2)`, topic, raw)
	return err
}

// SeedOtherTenantFleet seeds a second tenant inside the test database — a carrier, a route and a bus —
// as a tenant sharing `web` would have (INFRA-009). The API scopes every write to the session's
// tenant, so it cannot create another tenant's rows.
func SeedOtherTenantFleet(t *testing.T, helper *testhelpers.ApiTestHelper) (tenantID, routeID, vehicleID string) {
	t.Helper()
	ctx := context.Background()
	tenantID, companyID := uuid.NewString(), uuid.NewString()
	routeID, vehicleID = uuid.NewString(), uuid.NewString()
	statements := []struct {
		sql  string
		args []interface{}
	}{
		{`INSERT INTO tracking.transport_companies (id, tenant_id, name, registration_number, status)
		  VALUES ($1, $2, 'Other Transit', $3, 'active')`, []interface{}{companyID, tenantID, "REG-" + companyID[:8]}},
		{`INSERT INTO tracking.routes (id, company_id, route_name, origin_address, destination_address, direction, status)
		  VALUES ($1, $2, 'Other Route', 'A', 'B', 'outbound', 'active')`, []interface{}{routeID, companyID}},
		{`INSERT INTO tracking.vehicles (id, company_id, plate_number, vehicle_type, capacity, status)
		  VALUES ($1, $2, $3, 'bus', 20, 'active')`, []interface{}{vehicleID, companyID, "OTH-" + vehicleID[:6]}},
	}
	for _, s := range statements {
		if _, err := helper.DB().Execute(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seed the other tenant: %v", err)
		}
	}
	return tenantID, routeID, vehicleID
}

// VehiclePositionCount is how many stored points a vehicle has.
func VehiclePositionCount(t *testing.T, helper *testhelpers.ApiTestHelper, vehicleID string) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT count(*) FROM tracking.vehicle_positions WHERE vehicle_id = $1`, vehicleID).Scan(&n); err != nil {
		t.Fatalf("count positions: %v", err)
	}
	return n
}

// ============================================================================
// ROUTE SETUP AND IMPORT (TRACK-034, APP-009)
// ============================================================================

// SetRouteVehicle puts a route on a vehicle without the PATCH's route.changed.
func SetRouteVehicle(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, vehicleID string) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.routes SET vehicle_id = $2 WHERE id = $1`, routeID, vehicleID); err != nil {
		t.Fatalf("route vehicle: %v", err)
	}
}

// StopPlacePoint is a stop place's latitude and longitude: the API answers them rounded.
func StopPlacePoint(t *testing.T, helper *testhelpers.ApiTestHelper, id string) (float64, float64) {
	t.Helper()
	var lat, lng float64
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT ST_Y(location::geometry), ST_X(location::geometry) FROM tracking.stop_places WHERE id = $1`,
		id).Scan(&lat, &lng); err != nil {
		t.Fatalf("stop place point: %v", err)
	}
	return lat, lng
}

// CountStopPlaces counts the test tenant's places, which the paged list does not total cheaply.
func CountStopPlaces(t *testing.T, helper *testhelpers.ApiTestHelper) int {
	t.Helper()
	var n int
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT COUNT(*) FROM tracking.stop_places WHERE tenant_id = $1`, TestTenantID).Scan(&n); err != nil {
		t.Fatalf("count stop places: %v", err)
	}
	return n
}

// TripDriverOn is the driver a route's trip of one day was built with.
func TripDriverOn(t *testing.T, helper *testhelpers.ApiTestHelper, routeID, day string) string {
	t.Helper()
	var driver string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT driver_id::text FROM tracking.trips WHERE route_id = $1 AND service_date = $2`, routeID, day).Scan(&driver); err != nil {
		t.Fatalf("trip driver: %v", err)
	}
	return driver
}

// CompanyName is a seeded carrier's name, which an import names it by.
func CompanyName(t *testing.T, helper *testhelpers.ApiTestHelper, id string) string {
	t.Helper()
	var name string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT name FROM tracking.transport_companies WHERE id = $1`, id).Scan(&name); err != nil {
		t.Fatalf("company name: %v", err)
	}
	return name
}

// OrganizationName is a seeded organization's name, which an import names it by.
func OrganizationName(t *testing.T, helper *testhelpers.ApiTestHelper, id string) string {
	t.Helper()
	var name string
	if err := helper.DB().QueryRow(context.Background(),
		`SELECT name FROM tracking.organizations WHERE id = $1`, id).Scan(&name); err != nil {
		t.Fatalf("organization name: %v", err)
	}
	return name
}

// SetTripStopsDueAgo moves every stop of a trip to be due minutesAgo before now, so a test reads a
// late trip whatever the hour it runs at (TRACK-041).
func SetTripStopsDueAgo(t *testing.T, helper *testhelpers.ApiTestHelper, tripID string, minutesAgo int) {
	t.Helper()
	if _, err := helper.DB().Execute(context.Background(),
		`UPDATE tracking.trip_stops SET planned_at = now() - make_interval(mins => $2) WHERE trip_id = $1`, tripID, minutesAgo); err != nil {
		t.Fatalf("set trip stops due: %v", err)
	}
}
