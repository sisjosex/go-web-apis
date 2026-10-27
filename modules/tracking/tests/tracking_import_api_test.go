//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	geoRepos "josex/web/modules/geo/repositories"
	importModels "josex/web/modules/import/models"
)

// ============================================================================
// CSV IMPORT (APP-009)
// ============================================================================

// importResult reads a validate or an import answer: the engine lists only the rows with an error or
// a warning (IMPORT-002 D2).
type importResult struct {
	Issues     []importModels.RowResult `json:"issues"`
	FailedRows []importModels.RowResult `json:"failed_rows"`
	WarnedRows []importModels.RowResult `json:"warned_rows"`
	Run        *importModels.RunMeta    `json:"run"`
}

// rows is the file's n rows in order: a listed row as listed, any other one clean — valid on a dry
// run, created on the real one.
func (r importResult) rows(n int) []importModels.RowResult {
	clean := importModels.RowStatusValid
	if r.Run != nil {
		clean = importModels.RowStatusCreated
	}
	byLine := map[int]importModels.RowResult{}
	for _, listed := range [][]importModels.RowResult{r.Issues, r.FailedRows, r.WarnedRows} {
		for _, row := range listed {
			byLine[row.Line] = row
		}
	}
	out := make([]importModels.RowResult, n)
	for i := range out {
		row, ok := byLine[i+2]
		if !ok {
			row = importModels.RowResult{Line: i + 2, Status: clean}
		}
		out[i] = row
	}
	return out
}

func importTrackingCSV(t *testing.T, helper *testhelpers.ApiTestHelper, path, resource, csv string) importResult {
	t.Helper()
	files := []testhelpers.MultipartFile{{Field: "file", Filename: resource + ".csv", Content: []byte(csv)}}
	w := helper.DoMultipartRequest("POST", path, map[string]string{"resource": resource}, files, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("import %s returned %d: %s", resource, w.Code, w.Body.String())
	}
	var response importResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode import response: %v — %s", err, w.Body.String())
	}
	return response
}

func rowStatuses(response importResult, n int) []string {
	statuses := make([]string, 0, n)
	for _, row := range response.rows(n) {
		statuses = append(statuses, row.Status)
	}
	return statuses
}

// TestImportVehicles - three new plates are created; a short plate is a field error on its row, and
// a plate repeated in the file is a duplicate, on the dry run already.
func TestImportVehicles(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	company := CompanyName(t, helper, MainCompanyID)
	tag := uuid.New().String()[:5]
	csv := fmt.Sprintf("plate,seats,type,company\nIMP-%[1]s1,40,bus,%[2]s\nIMP-%[1]s2,12,van,%[2]s\nIMP-%[1]s3,4,car,%[2]s\nX,10,bus,%[2]s\nIMP-%[1]s1,40,bus,%[2]s\n", tag, company)

	dry := importTrackingCSV(t, helper, "/import/validate", "tracking_vehicles", csv)
	assert.Equal(t, []string{"valid", "valid", "valid", "invalid", "duplicate"}, rowStatuses(dry, 5))

	done := importTrackingCSV(t, helper, "/import", "tracking_vehicles", csv)
	assert.Equal(t, []string{"created", "created", "created", "failed", "skipped"}, rowStatuses(done, 5))
	assert.Contains(t, done.rows(5)[3].Errors, "tracking.import.required|plate")

	list := ListVehicles(t, helper, "?search=IMP-"+tag)
	assert.Len(t, list.Vehicles, 3)
}

// TestImportStopPlacesAndDocumentTypes - the two lookup-free resources: a place needs its point, a
// document type its code once.
func TestImportStopPlacesAndDocumentTypes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tag := uuid.New().String()[:5]

	places := importTrackingCSV(t, helper, "/import", "tracking_stop_places",
		fmt.Sprintf("stop,latitude,longitude,address\nPlaza %[1]s,-17.3935,-66.1570,Centro\nSin punto %[1]s,,,\n", tag))
	assert.Equal(t, []string{"created", "failed"}, rowStatuses(places, 2))

	code := "T" + tag
	types := importTrackingCSV(t, helper, "/import", "tracking_document_types",
		fmt.Sprintf("code,document,applies_to,warn_days_before,blocks_service\n%[1]s,SOAT,vehicle,30,si\n%[1]s,SOAT again,vehicle,30,no\n", code))
	assert.Equal(t, []string{"created", "skipped"}, rowStatuses(types, 2))
}

// TestImportRiders - riders of a school are students; a name repeated in the file is a duplicate.
func TestImportRiders(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	school := OrganizationName(t, helper, MainSchoolID)
	last := "Imp" + uuid.New().String()[:5]
	csv := fmt.Sprintf("first_name,last_name,organization,home_latitude,home_longitude,guardian_name\n"+
		"Lucia,%[1]s,%[2]s,-17.39,-66.15,Rosa\nLucia,%[1]s,%[2]s,,,\nMateo,%[1]s,Nowhere,,,\n", last, school)

	done := importTrackingCSV(t, helper, "/import", "tracking_riders", csv)
	assert.Equal(t, []string{"created", "skipped", "failed"}, rowStatuses(done, 3))
	assert.Contains(t, done.rows(3)[2].Errors, "tracking.import.organization-unknown|Nowhere")
}

// loadPlaces replaces geo.places with the geo module's fixture (Av. Aroma, Calle Sucre in Cochabamba).
func loadPlaces(t *testing.T, helper *testhelpers.ApiTestHelper) {
	t.Helper()
	file, err := os.Open("../../geo/tests/fixtures/places.geojsonl")
	if err != nil {
		t.Fatalf("open places fixture: %v", err)
	}
	defer func() { _ = file.Close() }()
	if _, err := geoRepos.NewPlacesRepository(helper.DB()).Import(context.Background(), file); err != nil {
		t.Fatalf("import places: %v", err)
	}
}

// TestImportRiders_Geocoded - TRACK-072 D3: an address the places search finds becomes an approximate
// home, warned and listed under geocoded; a nonsense one stays missing its location; moving the pin
// confirms it.
func TestImportRiders_Geocoded(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	loadPlaces(t, helper)
	school := OrganizationName(t, helper, MainSchoolID)
	last := "Geo" + uuid.New().String()[:5]
	csv := fmt.Sprintf("first_name,last_name,organization,address\n"+
		"Dora,%[1]s,%[2]s,Av. Aroma 123\nEmi,%[1]s,%[2]s,Zzqxw Vvkj\n", last, school)

	dry := importTrackingCSV(t, helper, "/import/validate", "tracking_riders", csv)
	assert.Contains(t, dry.rows(2)[0].Warnings, "tracking.import.geocoded")

	done := importTrackingCSV(t, helper, "/import", "tracking_riders", csv)
	assert.Equal(t, []string{"created", "created"}, rowStatuses(done, 2))
	assert.Equal(t, []string{"tracking.import.geocoded"}, done.rows(2)[0].Warnings)
	assert.Equal(t, []string{"tracking.import.missing-location"}, done.rows(2)[1].Warnings)

	geocoded := ListRiders(t, helper, "?geocoded=true&search="+last)
	if !assert.Len(t, geocoded.Riders, 1) {
		return
	}
	dora := geocoded.Riders[0]
	assert.Equal(t, "Dora", dora.FirstName)
	assert.True(t, dora.HomeGeocoded)
	if assert.NotNil(t, dora.HomeLatitude) {
		assert.InDelta(t, -17.396, *dora.HomeLatitude, 0.01)
	}

	w := helper.DoRequest("PATCH", "/tracking/riders/"+dora.ID.String(), map[string]interface{}{
		"home_latitude": -17.3961, "home_longitude": -66.1565,
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Empty(t, ListRiders(t, helper, "?geocoded=true&search="+last).Riders)
}

// TestImportRiders_Service - TRACK-043: a pinned row, an address-only row (imported, warned, listed
// under missing_location with its address) and an unknown stop (refused by name); the legs and group
// land on the rider.
func TestImportRiders_Service(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	school := OrganizationName(t, helper, MainSchoolID)
	last := "Srv" + uuid.New().String()[:5]
	csv := fmt.Sprintf("first_name,last_name,organization,home_latitude,home_longitude,address,service,group,stop\n"+
		"Ana,%[1]s,%[2]s,-17.39,-66.15,,solo ida,5to B,\n"+
		"Beto,%[1]s,%[2]s,,,Calle Jordán 123,vuelta,,\n"+
		"Ceci,%[1]s,%[2]s,,,,ambos,,Parada Inexistente\n", last, school)

	done := importTrackingCSV(t, helper, "/import", "tracking_riders", csv)
	assert.Equal(t, []string{"created", "created", "failed"}, rowStatuses(done, 3))
	assert.Contains(t, done.rows(3)[1].Warnings, "tracking.import.missing-location")
	assert.Contains(t, done.rows(3)[2].Errors, "tracking.import.stop-unknown|Parada Inexistente")

	missing := ListRiders(t, helper, "?missing_location=true&search="+last)
	if assert.Len(t, missing.Riders, 1) {
		assert.Equal(t, "Beto", missing.Riders[0].FirstName)
		assert.Equal(t, "Calle Jordán 123", *missing.Riders[0].Address)
		assert.Equal(t, "return", missing.Riders[0].ServiceLegs)
	}
	all := ListRiders(t, helper, "?search="+last)
	for _, r := range all.Riders {
		if r.FirstName == "Ana" {
			assert.Equal(t, "outbound", r.ServiceLegs)
			assert.Equal(t, "5to B", *r.GroupLabel)
		}
	}
}
