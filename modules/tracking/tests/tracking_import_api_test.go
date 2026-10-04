//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	importModels "josex/web/modules/import/models"
)

// ============================================================================
// CSV IMPORT (APP-009)
// ============================================================================

func importTrackingCSV(t *testing.T, helper *testhelpers.ApiTestHelper, path, resource, csv string) importModels.ImportResponse {
	t.Helper()
	files := []testhelpers.MultipartFile{{Field: "file", Filename: resource + ".csv", Content: []byte(csv)}}
	w := helper.DoMultipartRequest("POST", path, map[string]string{"resource": resource}, files, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("import %s returned %d: %s", resource, w.Code, w.Body.String())
	}
	var response importModels.ImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode import response: %v — %s", err, w.Body.String())
	}
	return response
}

func rowStatuses(response importModels.ImportResponse) []string {
	statuses := make([]string, 0, len(response.Rows))
	for _, row := range response.Rows {
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
	assert.Equal(t, []string{"valid", "valid", "valid", "invalid", "duplicate"}, rowStatuses(dry))

	done := importTrackingCSV(t, helper, "/import", "tracking_vehicles", csv)
	assert.Equal(t, []string{"created", "created", "created", "failed", "skipped"}, rowStatuses(done))
	assert.Contains(t, done.Rows[3].Errors, "tracking.import.required|plate")

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
	assert.Equal(t, []string{"created", "failed"}, rowStatuses(places))

	code := "T" + tag
	types := importTrackingCSV(t, helper, "/import", "tracking_document_types",
		fmt.Sprintf("code,document,applies_to,warn_days_before,blocks_service\n%[1]s,SOAT,vehicle,30,si\n%[1]s,SOAT again,vehicle,30,no\n", code))
	assert.Equal(t, []string{"created", "skipped"}, rowStatuses(types))
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
	assert.Equal(t, []string{"created", "skipped", "failed"}, rowStatuses(done))
	assert.Contains(t, done.Rows[2].Errors, "tracking.import.organization-unknown|Nowhere")
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
	assert.Equal(t, []string{"created", "created", "failed"}, rowStatuses(done))
	assert.Contains(t, done.Rows[1].Warnings, "tracking.import.missing-location")
	assert.Contains(t, done.Rows[2].Errors, "tracking.import.stop-unknown|Parada Inexistente")

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
