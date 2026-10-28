//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// TRACK-074: the riders, drivers, organizations and companies lists carry the figures their screens show.

func listPeople(t *testing.T, helper *testhelpers.ApiTestHelper, path string, into any) {
	t.Helper()
	w := helper.DoRequest("GET", path, nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), into), w.Body.String())
}

// TestRiders_TodayStatusAndCounts - a rider with an absence today reads absent and one still to be
// picked up pending; the type filter keeps the other type's chip count; an empty page keeps them too.
func TestRiders_TodayStatusAndCounts(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tag := "Lista" + uuid.New().String()[:6]
	absentRoute, absentID, _ := todaysTrip(t, helper)
	_, pendingID, _ := todaysTrip(t, helper)
	RenameRider(t, helper, absentID, tag)
	RenameRider(t, helper, pendingID, tag)
	today := dayOffset(t, "UTC", 0)
	// The route runs on UTC and the client on Lima: in the evening those are two days, and the trip's
	// day and the client's day must both be covered for the rider to read absent at any hour.
	from, to := today, dayOffset(t, "America/Lima", 0)
	if to < from {
		from, to = to, from
	}
	postAbsence(t, helper, absentID, map[string]interface{}{"date_from": from, "date_to": to}, http.StatusCreated)
	// What the route.changed worker does next: the planned trip is rebuilt without the absent rider.
	materialiseRoute(t, helper, absentRoute, today, today)
	employeeID := createRider(t, helper, map[string]interface{}{"rider_type": "employee"})
	RenameRider(t, helper, employeeID, tag)

	var all models.ListRidersResponse
	listPeople(t, helper, "/tracking/riders?search="+tag, &all)
	assert.Equal(t, models.RiderCounts{All: 3, Student: 2, Employee: 1, Unassigned: 1}, all.Counts)
	status := map[string]*string{}
	for _, r := range all.Riders {
		status[r.ID.String()] = r.TodayStatus
		if assert.NotNil(t, r.GuardianApp) {
			assert.False(t, *r.GuardianApp, "no guardian opened the tracking")
		}
	}
	if assert.NotNil(t, status[absentID]) {
		assert.Equal(t, "absent", *status[absentID])
	}
	if assert.NotNil(t, status[pendingID]) {
		assert.Equal(t, "pending", *status[pendingID])
	}
	assert.Nil(t, status[employeeID], "no pickup today")

	var students models.ListRidersResponse
	listPeople(t, helper, "/tracking/riders?rider_type=student&search="+tag, &students)
	assert.Len(t, students.Riders, 2)
	assert.Equal(t, int64(1), students.Counts.Employee)

	var unassigned models.ListRidersResponse
	listPeople(t, helper, "/tracking/riders?unassigned=true&search="+tag, &unassigned)
	if assert.Len(t, unassigned.Riders, 1) {
		assert.Equal(t, employeeID, unassigned.Riders[0].ID.String())
	}

	var empty models.ListRidersResponse
	listPeople(t, helper, "/tracking/riders?page=9&search="+tag, &empty)
	assert.Empty(t, empty.Riders)
	assert.Equal(t, int64(3), empty.Counts.All)
}

// TestDrivers_AppStatusAndSummary - D1: an account that synced an op today is active, one that never
// did is invited, a driver without an account has none; the summary counts all three.
func TestDrivers_AppStatusAndSummary(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	surname := "Chofer " + uuid.New().String()[:6]
	create := func(account bool) string {
		body := ValidDriverBody()
		body["last_name"] = surname
		if account {
			body["account"] = map[string]interface{}{"email": newAccessEmail()}
		}
		w := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		return ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	}
	active, invited, none := create(true), create(true), create(false)
	RecordDriverOp(t, helper, active)

	var res models.ListDriversResponse
	listPeople(t, helper, "/tracking/drivers?search="+url.QueryEscape(surname), &res)
	statuses := map[string]string{}
	for _, d := range res.Drivers {
		require.NotNil(t, d.AppStatus)
		statuses[d.ID.String()] = *d.AppStatus
	}
	assert.Equal(t, map[string]string{active: "active", invited: "invited", none: "none"}, statuses)
	assert.Equal(t, models.DriverSummary{Total: 3, AppActive: 1, DocsDue: 0, WithoutRouteToday: 3}, res.Summary)
}

// TestOrganizationsAndCompanies_Figures - the kind filter keeps the other kinds' counts; a carrier with
// two vehicles reads vehicles_count 2 and its fleet figures.
func TestOrganizationsAndCompanies_Figures(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	all := ListOrganizations(t, helper, "")
	schools := ListOrganizations(t, helper, "?kind=school")
	assert.Equal(t, all.Counts, schools.Counts)
	assert.Equal(t, schools.Counts.School, schools.TotalCount)
	for _, o := range schools.Organizations {
		assert.NotNil(t, o.RoutesCount)
		assert.NotNil(t, o.RidersCount)
		assert.NotNil(t, o.AbsentToday)
		assert.NotNil(t, o.ShiftsCount)
	}

	companyID := createFleetCompany(t, helper)
	CreateTestVehicleOf(t, helper, companyID)
	CreateTestVehicleOf(t, helper, companyID)
	name := CompanyName(t, helper, companyID)
	res := ListCompanies(t, helper, "?search="+url.QueryEscape(name))
	require.Len(t, res.Companies, 1)
	c := res.Companies[0]
	assert.Equal(t, int64(2), *c.VehiclesCount)
	assert.Equal(t, int64(0), *c.DriversCount)
	assert.Equal(t, int64(0), *c.ActiveRoutesCount)
	assert.Equal(t, int64(0), *c.DocumentsDue)
}

// driversWorkbook is a carrier's drivers sheet with its own Spanish headings.
func driversWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	_ = book.SetSheetRow(sheet, "A1", &[]any{"Nombre", "Apellido", "Teléfono", "Correo", "Licencia", "Categoría", "Vence licencia", "Empresa"})
	for i, row := range rows {
		cellName, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = book.SetSheetRow(sheet, cellName, &row)
	}
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)
	return buf.Bytes()
}

func importDrivers(t *testing.T, helper *testhelpers.ApiTestHelper, book []byte, onExisting string) importResult {
	t.Helper()
	files := []testhelpers.MultipartFile{{Field: "file", Filename: "conductores.xlsx", Content: book}}
	w := helper.DoMultipartRequest("POST", "/import", map[string]string{"resource": "tracking_drivers", "on_existing": onExisting}, files, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var response importResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response), w.Body.String())
	return response
}

// TestImportDrivers - three rows with Spanish headings become three drivers, the one with an email
// invited to the app; the same file again with on_existing=update changes a phone instead of repeating.
func TestImportDrivers(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	company := CompanyName(t, helper, MainCompanyID)
	tag := uuid.New().String()[:6]
	rows := [][]any{
		{"Ana", "Import " + tag, "70000001", newAccessEmail(), "LI1-" + tag, "B", "01/05/2027", company},
		{"Luis", "Import " + tag, "70000002", "", "LI2-" + tag, "C", "", company},
		{"Rosa", "Import " + tag, "70000003", "", "LI3-" + tag, "", "", company},
	}
	done := importDrivers(t, helper, driversWorkbook(t, rows), "skip")
	assert.Equal(t, []string{"created", "created", "created"}, rowStatuses(done, 3))

	var res models.ListDriversResponse
	listPeople(t, helper, "/tracking/drivers?search="+url.QueryEscape("Import "+tag), &res)
	require.Len(t, res.Drivers, 3)
	byLicense := map[string]*models.Driver{}
	for _, d := range res.Drivers {
		byLicense[*d.LicenseNumber] = d
	}
	assert.Equal(t, "invited", *byLicense["LI1-"+tag].AppStatus)
	assert.Equal(t, "none", *byLicense["LI2-"+tag].AppStatus)

	rows[1][2] = "79999999"
	again := importDrivers(t, helper, driversWorkbook(t, rows), "update")
	if assert.NotNil(t, again.Run) {
		assert.Equal(t, 3, again.Run.Updated)
		assert.Equal(t, 0, again.Run.Created)
	}
	listPeople(t, helper, "/tracking/drivers?search="+url.QueryEscape("Import "+tag), &res)
	require.Len(t, res.Drivers, 3)
	for _, d := range res.Drivers {
		if *d.LicenseNumber == "LI2-"+tag {
			assert.Equal(t, "79999999", *d.Phone)
		}
	}
}
