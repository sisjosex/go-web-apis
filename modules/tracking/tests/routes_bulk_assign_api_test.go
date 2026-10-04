//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TRACK-039: many riders of a destination added to a route at once, never twice.

// TestBulkAssign_SkipsOverlapAndNoHome - three homed riders, one already on another outbound route,
// and one without a home: two go in (on the return too), the other two are skipped with their reason.
func TestBulkAssign_SkipsOverlapAndNoHome(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	out, ret := createPair(t, helper, "")
	outID, retID := ExtractID(t, out), ExtractID(t, ret)

	a := homedRider(t, helper, -17.3801, -66.1601)
	b := homedRider(t, helper, -17.3811, -66.1611)
	busy := homedRider(t, helper, -17.3821, -66.1621)
	noHome := CreateTestRider(t, helper)
	CreateAssignment(t, helper, AssignmentBody(busy, CreateTestRoute(t, helper), WeekdaysMask))

	// The list for the route names the busy rider's route, and "unassigned" leaves it out.
	w := helper.DoRequest("GET", fmt.Sprintf("/tracking/riders?route_id=%s&search=Mamani&page_size=100", outID), nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	named := map[string]interface{}{}
	for _, row := range ParseResponse(t, w.Body.Bytes())["riders"].([]interface{}) {
		r := row.(map[string]interface{})
		named[r["id"].(string)] = r["assigned_route_name"]
	}
	assert.NotNil(t, named[busy], "the busy rider names the route it rides")
	assert.Nil(t, named[a])
	w = helper.DoRequest("GET", fmt.Sprintf("/tracking/riders?route_id=%s&unassigned=true&search=Mamani&page_size=100", outID), nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	for _, row := range ParseResponse(t, w.Body.Bytes())["riders"].([]interface{}) {
		assert.NotEqual(t, busy, row.(map[string]interface{})["id"])
	}

	w = helper.DoRequest("POST", "/tracking/routes/"+outID+"/assignments/bulk", map[string]interface{}{
		"rider_ids": []string{a, b, busy, noHome},
	}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	body := ParseResponse(t, w.Body.Bytes())
	assert.EqualValues(t, 2, body["assigned_count"])
	reasons := map[string]interface{}{}
	for _, row := range body["results"].([]interface{}) {
		r := row.(map[string]interface{})
		reasons[r["rider_id"].(string)] = r["reason"]
	}
	assert.Equal(t, "overlap", reasons[busy])
	assert.Equal(t, "no-home", reasons[noHome])
	assert.Nil(t, reasons[a])

	retRiders := map[string]bool{}
	for _, row := range listAssignments(t, helper, "route_id="+retID)["assignments"].([]interface{}) {
		retRiders[row.(map[string]interface{})["rider_id"].(string)] = true
	}
	assert.True(t, retRiders[a] && retRiders[b], "the return carries the same two")
	assert.Len(t, retRiders, 2)
}

// TestRiderGroup_SetFilteredListed - a rider's group is saved, filters the list and is offered.
func TestRiderGroup_SetFilteredListed(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	body := riderWithGuardians(nil)
	delete(body, "guardians")
	body["group_label"] = "5to B"
	w := helper.DoRequest("POST", "/tracking/riders", body, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	riderID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))

	w = helper.DoRequest("GET", "/tracking/riders?group=5to%20B&page_size=100", nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	rows := ParseResponse(t, w.Body.Bytes())["riders"].([]interface{})
	if assert.NotEmpty(t, rows) {
		for _, row := range rows {
			assert.Equal(t, "5to B", row.(map[string]interface{})["group_label"])
		}
	}

	w = helper.DoRequest("GET", "/tracking/riders/groups?organization_id="+MainSchoolID, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Contains(t, ParseResponse(t, w.Body.Bytes())["groups"], "5to B")

	// An empty group clears it; leaving the field out keeps it.
	w = helper.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{"group_label": ""}, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	assert.Nil(t, ParseResponse(t, w.Body.Bytes())["group_label"])
}
