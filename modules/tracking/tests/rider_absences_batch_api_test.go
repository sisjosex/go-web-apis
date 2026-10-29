//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"josex/web/modules/tracking/models"
)

// ---- MOBILE-038 D2: several runs of days in one call ----

// TestRiderAbsencesBatch_AllOrNothing - a guardian reports Mon–Tue and Thu of a later week in one call:
// two absences; a batch whose second range is past the cut-off answers 409 naming that day and creates
// none, the first range included; another guardian's rider is not found.
func TestRiderAbsencesBatch_AllOrNothing(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	w := helper.DoRequest("POST", "/tracking/organizations", OrganizationBody(ValidOrganizationDto()), map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: %d %s", w.Code, w.Body.String())
	}
	organizationID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	w = helper.DoRequest("PATCH", "/tracking/organizations/"+organizationID, map[string]interface{}{"absence_cutoff_min": 1440}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	riderID := createRider(t, helper, map[string]interface{}{"organization_id": organizationID})
	LinkPortalGuardian(t, helper, riderID)

	// A pickup 23 hours away: its day's cut-off passed an hour ago.
	pickupAt := time.Now().UTC().Add(23 * time.Hour)
	closedDay := pickupAt.Format(time.DateOnly)
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": pickupAt.Format("15:04"), "valid_from": "2026-01-01",
	})
	assignRider(t, helper, riderID, routeID)
	materialiseRoute(t, helper, routeID, closedDay, closedDay)

	portal := SetupPortalTest(t)
	defer portal.Close()
	day := func(offset int) string { return time.Now().UTC().AddDate(0, 0, offset).Format(time.DateOnly) }
	batch := func(rider string, ranges ...[2]string) *httpResult {
		t.Helper()
		body := []map[string]string{}
		for _, r := range ranges {
			body = append(body, map[string]string{"date_from": r[0], "date_to": r[1]})
		}
		w := portal.DoRequest("POST", "/tracking/riders/"+rider+"/absences/batch",
			map[string]interface{}{"ranges": body, "reason": "Viaje"}, map[string]string{})
		return &httpResult{code: w.Code, body: w.Body.String(), raw: w.Body.Bytes()}
	}
	absences := func() int {
		t.Helper()
		w := portal.DoRequest("GET", "/tracking/riders/"+riderID+"/absences?from="+day(0)+"&to="+day(40), nil, map[string]string{})
		if w.Code != http.StatusOK {
			t.Fatalf("list absences: %d %s", w.Code, w.Body.String())
		}
		var list []models.RiderAbsence
		decodeBody(t, w, &list)
		return len(list)
	}

	ok := batch(riderID, [2]string{day(20), day(21)}, [2]string{day(23), day(23)})
	if assert.Equal(t, http.StatusCreated, ok.code, ok.body) {
		var created struct {
			Absences []struct {
				DateFrom string `json:"date_from"`
				DateTo   string `json:"date_to"`
			} `json:"absences"`
		}
		if err := json.Unmarshal(ok.raw, &created); err != nil {
			t.Fatalf("decode batch: %v", err)
		}
		if assert.Len(t, created.Absences, 2) {
			assert.Equal(t, day(20), created.Absences[0].DateFrom)
			assert.Equal(t, day(21), created.Absences[0].DateTo)
			assert.Equal(t, day(23), created.Absences[1].DateFrom)
		}
	}
	assert.Equal(t, 2, absences())

	refused := batch(riderID, [2]string{day(30), day(30)}, [2]string{closedDay, closedDay})
	assert.Equal(t, http.StatusConflict, refused.code, refused.body)
	assert.Contains(t, refused.body, "tracking.absence.cutoff")
	assert.Contains(t, refused.body, closedDay, "the refusal names the day it hit")
	assert.Equal(t, 2, absences(), "nothing of a refused batch stays")

	other := batch(TestRiderJaneID, [2]string{day(20), day(20)})
	assert.Equal(t, http.StatusNotFound, other.code, "another guardian's rider: %s", other.body)
}

// httpResult is one response, kept past its recorder.
type httpResult struct {
	code int
	body string
	raw  []byte
}
