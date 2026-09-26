// filepath: modules/tracking/tests/helpers.go
//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
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

// UploadDocumentFile sends one file to PUT /tracking/documents/:id/file and returns the recorder,
// so a test can assert the status as well as the body.
func UploadDocumentFile(_ *testing.T, helper *testhelpers.ApiTestHelper, documentID, filename string, content []byte) *httptest.ResponseRecorder {
	return helper.DoMultipartRequest("PUT", "/tracking/documents/"+documentID+"/file", nil,
		[]testhelpers.MultipartFile{{Field: "file", Filename: filename, Content: content}},
		map[string]string{})
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

// CreateAssignment posts an assignment and returns the response body's assignment.
func CreateAssignment(t *testing.T, helper *testhelpers.ApiTestHelper, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	w := helper.DoRequest("POST", "/tracking/assignments", body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("assign rider: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	return ParseResponse(t, w.Body.Bytes())["assignment"].(map[string]interface{})
}

// CreateTestRider creates a rider in MainSchoolID and returns its id
func CreateTestRider(t *testing.T, helper *testhelpers.ApiTestHelper) string {
	dto := ValidRiderDto()
	body := map[string]interface{}{
		"organization_id": dto.OrganizationID,
		"rider_type":      dto.RiderType,
		"first_name":      dto.FirstName,
		"last_name":       dto.LastName,
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

// CleanTrackingDatabase cleans only tracking tables while preserving auth and system data
// This is the proper cleanup for tracking-specific tests
func CleanTrackingDatabase(helper *testhelpers.ApiTestHelper) error {
	return helper.CleanDatabaseForSchemas("tracking")
}
