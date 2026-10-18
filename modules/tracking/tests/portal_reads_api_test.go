//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// TRACK-061: the guardian's reads — the riders across operators, the rider's day, the history, the
// absence cut-offs.

// ---- step 1: /mobile/portal/me ----

// TestPortalMe_RidersAcrossOperators - D1: a guardian of one rider in each of two tenants reads both
// in one call, the newest membership first, each rider with its client's kind and type.
func TestPortalMe_RidersAcrossOperators(t *testing.T) {
	operator := SetupTrackingTest(t)
	defer operator.Close()
	slug, riderID := SeedPortalSecondTenant(t, operator)

	portal := SetupPortalTest(t)
	defer portal.Close()
	w := portal.DoRequest("GET", "/mobile/portal/me", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d %s", w.Code, w.Body.String())
	}
	var me models.PortalMe
	decodeBody(t, w, &me)
	assert.NotEmpty(t, me.User.FirstName)
	if !assert.Len(t, me.Tenants, 2, w.Body.String()) {
		return
	}
	second, first := me.Tenants[0], me.Tenants[1]
	assert.Equal(t, slug, second.Slug, "the newest membership first")
	assert.Equal(t, "test-company", first.Slug)
	assert.False(t, second.Error)
	assert.False(t, first.Error)
	if assert.Len(t, second.Riders, 1) {
		assert.Equal(t, riderID, second.Riders[0].RiderID.String())
		assert.Equal(t, "company", *second.Riders[0].OrganizationKind)
		assert.Equal(t, "employee", *second.Riders[0].RiderType)
	}
	ids := []string{}
	for _, r := range first.Riders {
		ids = append(ids, r.RiderID.String())
	}
	assert.Contains(t, ids, TestRiderJohnID)

	// The home's own read scans the two new columns too.
	w = portal.DoRequest("GET", "/tracking/riders/today", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"organization_kind":"school"`)
}

// TestPortalMe_NoPortalMembershipIsRefused - an account that is a guardian nowhere → 403.
func TestPortalMe_NoPortalMembershipIsRefused(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	w := helper.DoRequest("GET", "/mobile/portal/me", nil, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// ---- step 2: the rider's day ----

// todayTasks answers the rider's pickup and dropoff task ids on a trip.
func todayTasks(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, riderID string) (string, string) {
	t.Helper()
	var detail models.TripDetail
	decodeBody(t, helper.DoRequest("GET", "/tracking/trips/"+tripID, nil, map[string]string{}), &detail)
	return taskOf(t, detail, riderID, "pickup"), taskOf(t, detail, riderID, "dropoff")
}

func doneTask(t *testing.T, helper *testhelpers.ApiTestHelper, taskID string) {
	t.Helper()
	tripRequest(t, helper, "POST", "/tracking/trip-stop-tasks/"+taskID+"/done",
		map[string]interface{}{"client_op_id": uuid.NewString()}, http.StatusOK)
}

func riderToday(t *testing.T, helper *testhelpers.ApiTestHelper, riderID string) (models.RiderToday, string) {
	t.Helper()
	w := helper.DoRequest("GET", "/tracking/riders/"+riderID+"/today", nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("rider today: %d %s", w.Code, w.Body.String())
	}
	var day models.RiderToday
	decodeBody(t, w, &day)
	return day, w.Header().Get("ETag")
}

// TestRiderToday_Timeline - D2: the morning trip, ridden and completed, stays on the day with both
// stops done; the afternoon one in progress carries the ETA to the pickup, until the rider is
// dropped off. The same day again with its ETag → 304; another guardian → 404.
func TestRiderToday_Timeline(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	riderID := CreateTestRider(t, helper)
	LinkPortalGuardian(t, helper, riderID)
	today := dayOffset(t, "UTC", 0)

	morning, mStops := cochabambaRoute(t, helper)
	afternoon, aStops := cochabambaRoute(t, helper)
	SetRouteDirection(t, helper, afternoon, "inbound")
	for route, stops := range map[string][]string{morning: mStops, afternoon: aStops} {
		body := AssignmentBody(riderID, route, EveryDayMask)
		body["pickup_stop_place_id"] = stops[0]
		body["dropoff_stop_place_id"] = stops[1]
		CreateAssignment(t, helper, body)
	}

	materialiseRoute(t, helper, morning, today, today)
	morningTrip, _ := tripOn(t, helper, morning, today, "07:00")
	tripRequest(t, helper, "PATCH", "/tracking/trips/"+morningTrip, map[string]interface{}{"vehicle_id": CreateTestVehicleOf(t, helper, MainCompanyID)}, http.StatusOK)
	tripRequest(t, helper, "POST", "/tracking/trips/"+morningTrip+"/start", nil, http.StatusOK)
	pickup, dropoff := todayTasks(t, helper, morningTrip, riderID)
	doneTask(t, helper, pickup)
	doneTask(t, helper, dropoff)
	CompleteTrip(t, helper, morningTrip)
	MoveTripStart(t, helper, morningTrip, -600)

	afternoonTrip, _ := startCochabambaTrip(t, helper, afternoon)
	MoveTripStart(t, helper, afternoonTrip, -5)

	portal := SetupPortalTest(t)
	defer portal.Close()
	day, _ := riderToday(t, portal, riderID)
	assert.Equal(t, dayOffset(t, day.Timezone, 0), day.Date, "today in the rider's organization's zone")
	assert.Equal(t, "school", day.Organization.Kind)
	if assert.NotNil(t, day.Company) {
		assert.NotEmpty(t, day.Company.Name)
	}
	if !assert.Len(t, day.Trips, 2) {
		return
	}
	first, second := day.Trips[0], day.Trips[1]
	assert.Equal(t, morningTrip, first.TripID.String())
	assert.Equal(t, "completed", first.Status)
	if assert.NotNil(t, first.Pickup) && assert.NotNil(t, first.Dropoff) {
		assert.NotNil(t, first.Pickup.DoneAt)
		assert.NotNil(t, first.Dropoff.DoneAt)
		assert.Nil(t, first.Pickup.EtaAt)
	}
	assert.Equal(t, afternoonTrip, second.TripID.String())
	assert.Equal(t, "in_progress", second.Status)
	if assert.NotNil(t, second.Pickup) {
		assert.Equal(t, "pending", second.Pickup.TaskStatus)
		assert.NotNil(t, second.Pickup.EtaAt, "the ETA to the rider's pickup while they wait")
		assert.NotNil(t, second.Pickup.Lat)
	}
	if assert.NotNil(t, second.StopsLeft) {
		assert.Equal(t, int32(0), *second.StopsLeft)
	}

	pickup, dropoff = todayTasks(t, helper, afternoonTrip, riderID)
	doneTask(t, helper, pickup)
	doneTask(t, helper, dropoff)
	day, etag := riderToday(t, portal, riderID)
	if assert.Len(t, day.Trips, 2) && assert.NotNil(t, day.Trips[1].Dropoff) {
		assert.Nil(t, day.Trips[1].Pickup.EtaAt, "no ETA once the rider is off")
		assert.Nil(t, day.Trips[1].Dropoff.EtaAt)
		assert.Nil(t, day.Trips[1].StopsLeft)
	}
	if assert.NotEmpty(t, etag) {
		w := portal.DoRequest("GET", "/tracking/riders/"+riderID+"/today", nil, map[string]string{"If-None-Match": etag})
		assert.Equal(t, http.StatusNotModified, w.Code, w.Body.String())
	}

	stranger := SetupUnlinkedPortalTest(t)
	defer stranger.Close()
	w := stranger.DoRequest("GET", "/tracking/riders/"+riderID+"/today", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// ---- step 3: history ----

// setRiderTasks writes the rider's task statuses on a trip straight to the table, done_at with done.
func setRiderTasks(t *testing.T, helper *testhelpers.ApiTestHelper, tripID, riderID, kind, status string) {
	t.Helper()
	execSQL(t, helper, `UPDATE tracking.trip_stop_tasks k
		SET status = $4::VARCHAR, done_at = CASE WHEN $4::VARCHAR = 'done' THEN now() END
		FROM tracking.trip_stops s
		WHERE s.id = k.trip_stop_id AND s.trip_id = $1 AND k.subject_id = $2 AND k.kind = $3`,
		tripID, riderID, kind, status)
}

// TestRiderHistory_DaysNewestFirst - D3: a day ridden, a day not shown up, a day reported absent →
// three rows newest first with those statuses and the ridden day's times; 40 days apart → 400.
func TestRiderHistory_DaysNewestFirst(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	routeID, riderID := createTripRoute(t, helper, "07:00")
	d1, d2, d3 := dayOffset(t, "UTC", 1), dayOffset(t, "UTC", 2), dayOffset(t, "UTC", 3)
	materialiseRoute(t, helper, routeID, d1, d3)

	trip1, _ := tripOn(t, helper, routeID, d1, "07:00")
	setRiderTasks(t, helper, trip1, riderID, "pickup", "done")
	setRiderTasks(t, helper, trip1, riderID, "dropoff", "done")
	trip2, _ := tripOn(t, helper, routeID, d2, "07:00")
	setRiderTasks(t, helper, trip2, riderID, "pickup", "no_show")
	postAbsence(t, helper, riderID, map[string]interface{}{"date_from": d3, "date_to": d3, "reason": "Sick"}, http.StatusCreated)

	w := helper.DoRequest("GET", "/tracking/riders/"+riderID+"/history?from="+d1+"&to="+d3, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	var history models.RiderHistory
	decodeBody(t, w, &history)
	assert.Equal(t, d1, history.From)
	assert.Equal(t, d3, history.To)
	if !assert.Len(t, history.Days, 3, w.Body.String()) {
		return
	}
	statuses := []string{}
	for _, d := range history.Days {
		statuses = append(statuses, d.Date+":"+d.Status)
	}
	assert.Equal(t, []string{d3 + ":absent", d2 + ":no_show", d1 + ":rode"}, statuses)
	if assert.NotNil(t, history.Days[0].Absence) {
		assert.Equal(t, "Sick", *history.Days[0].Absence.Reason)
	}
	if assert.Len(t, history.Days[2].Legs, 1) {
		leg := history.Days[2].Legs[0]
		assert.Equal(t, trip1, leg.TripID.String())
		assert.NotNil(t, leg.PickupAt)
		assert.NotNil(t, leg.DropoffAt)
		assert.NotNil(t, leg.PickupStop)
	}

	w = helper.DoRequest("GET", "/tracking/riders/"+riderID+"/history?from="+dayOffset(t, "UTC", -40)+"&to="+dayOffset(t, "UTC", 0), nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.rider.history-range")
}

// ---- step 4: absence cut-offs ----

// TestRiderAbsenceCutoffs_AgreeWithCreate - a 1440-minute cut-off and a pickup a day less an hour away:
// that day is closed and reporting it is 409 cutoff; the next day is open and reporting it is 201.
func TestRiderAbsenceCutoffs_AgreeWithCreate(t *testing.T) {
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

	// The pickup lands 23 hours from now, so its cut-off passed an hour ago; the next day's is 23 hours away.
	pickupAt := time.Now().UTC().Add(23 * time.Hour)
	closedDay, openDay := pickupAt.Format(time.DateOnly), pickupAt.AddDate(0, 0, 1).Format(time.DateOnly)
	routeID := CreateTestRoute(t, helper)
	PublishRouteVersion(t, helper, routeID, "2026-01-01", []map[string]interface{}{
		{"stop_place_id": CentralStationID, "sequence": 1, "planned_offset_min": 0},
		{"stop_place_id": SchoolAStopID, "sequence": 2, "planned_offset_min": 30},
	})
	CreateRouteSchedule(t, helper, routeID, map[string]interface{}{
		"days_of_week": EveryDayMask, "start_time": pickupAt.Format("15:04"), "valid_from": "2026-01-01",
	})
	assignRider(t, helper, riderID, routeID)
	materialiseRoute(t, helper, routeID, closedDay, openDay)

	portal := SetupPortalTest(t)
	defer portal.Close()
	w = portal.DoRequest("GET", "/tracking/riders/"+riderID+"/absence-cutoffs?from="+closedDay+"&to="+openDay, nil, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("absence cutoffs: %d %s", w.Code, w.Body.String())
	}
	var cutoffs models.RiderAbsenceCutoffs
	decodeBody(t, w, &cutoffs)
	assert.Equal(t, int32(1440), cutoffs.CutoffMin)
	if assert.Len(t, cutoffs.Days, 2) {
		assert.Equal(t, closedDay, cutoffs.Days[0].Date)
		assert.True(t, cutoffs.Days[0].HasTrip)
		assert.True(t, cutoffs.Days[0].Closed)
		assert.NotNil(t, cutoffs.Days[0].CutoffAt)
		assert.True(t, cutoffs.Days[1].HasTrip)
		assert.False(t, cutoffs.Days[1].Closed)
	}

	w = portal.DoRequest("POST", "/tracking/riders/"+riderID+"/absences", map[string]interface{}{"date_from": closedDay, "date_to": closedDay}, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.absence.cutoff")
	postAbsence(t, portal, riderID, map[string]interface{}{"date_from": openDay, "date_to": openDay}, http.StatusCreated)

	// Defaults answer two weeks; more than 31 days is refused.
	w = portal.DoRequest("GET", "/tracking/riders/"+riderID+"/absence-cutoffs", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	decodeBody(t, w, &cutoffs)
	assert.Len(t, cutoffs.Days, 14)
	w = portal.DoRequest("GET", "/tracking/riders/"+riderID+"/absence-cutoffs?from="+closedDay+"&to="+pickupAt.AddDate(0, 0, 40).Format(time.DateOnly), nil, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
