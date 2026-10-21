//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/testhelpers"
	trackingJobs "josex/web/modules/tracking/jobs"
	"josex/web/modules/tracking/models"
)

// TRACK-067: an organization's own admin manages it on the web, a viewer only reads, a supervisor
// with groups sees only those groups' riders and the trips carrying them.

// setOrgUserMembership turns the seeded organization user's one membership (MainSchoolID, admin)
// into role with groups for this test, and gives its tenant role the organizations capabilities.
func setOrgUserMembership(t *testing.T, helper *testhelpers.ApiTestHelper, role string, groups []string) {
	t.Helper()
	GrantOrganizationRole(t, helper, "tracking:organizations:read")
	GrantOrganizationRole(t, helper, "tracking:organizations:write")
	execSQL(t, helper, `UPDATE tracking.organization_members m SET role = $2, groups = $3
		FROM auth.users u WHERE u.id = m.user_id AND u.email = 'orguser@test.local' AND m.organization_id = $1`,
		MainSchoolID, role, groups)
	t.Cleanup(func() {
		execSQL(t, helper, `UPDATE tracking.organization_members m SET role = 'admin', groups = NULL
			FROM auth.users u WHERE u.id = m.user_id AND u.email = 'orguser@test.local' AND m.organization_id = $1`, MainSchoolID)
	})
}

// groupRiding is a fresh MainSchoolID rider of group on today's started Cochabamba trip, on a fresh
// vehicle that has just reported. It answers the rider, the trip and the vehicle.
func groupRiding(t *testing.T, helper *testhelpers.ApiTestHelper, group string) (string, string, string) {
	t.Helper()
	routeID, stops := cochabambaRoute(t, helper)
	riderID := createRider(t, helper, map[string]interface{}{"group_label": group})
	body := AssignmentBody(riderID, routeID, EveryDayMask)
	body["pickup_stop_place_id"] = stops[0]
	body["dropoff_stop_place_id"] = stops[1]
	CreateAssignment(t, helper, body)
	tripID, vehicleID := startCochabambaTrip(t, helper, routeID)
	return riderID, tripID, vehicleID
}

// TestOrganizationSelf_AdminManagesItsOwn - an organization admin lists exactly its organization with
// its role, edits it, invites a supervisor with groups, re-roles and removes it; another organization
// is not found.
func TestOrganizationSelf_AdminManagesItsOwn(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	setOrgUserMembership(t, staff, "admin", nil)
	foreign := createOrganization(t, staff)
	org := SetupOrganizationTest(t)
	defer org.Close()

	var list models.ListOrganizationsResponse
	decodeBody(t, org.DoRequest("GET", "/tracking/organizations", nil, map[string]string{}), &list)
	if assert.Len(t, list.Organizations, 1) {
		assert.Equal(t, MainSchoolID, list.Organizations[0].ID.String())
		if assert.NotNil(t, list.Organizations[0].MyRole) {
			assert.Equal(t, "admin", *list.Organizations[0].MyRole)
		}
	}
	assert.EqualValues(t, 1, list.TotalCount)

	var own models.Organization
	decodeBody(t, org.DoRequest("GET", "/tracking/organizations/"+MainSchoolID, nil, map[string]string{}), &own)
	if assert.NotNil(t, own.MyRole) {
		assert.Equal(t, "admin", *own.MyRole)
	}
	w := org.DoRequest("PATCH", "/tracking/organizations/"+MainSchoolID, map[string]interface{}{"name": own.Name}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusNotFound, org.DoRequest("GET", "/tracking/organizations/"+foreign, nil, map[string]string{}).Code)
	assert.Equal(t, http.StatusNotFound, org.DoRequest("PATCH", "/tracking/organizations/"+foreign,
		map[string]interface{}{"name": "Taken over"}, map[string]string{}).Code)
	assert.Equal(t, http.StatusNotFound, org.DoRequest("GET", "/tracking/organizations/"+foreign+"/members", nil, map[string]string{}).Code)

	body := accessPerson(newAccessEmail())
	body["role"] = "supervisor"
	body["groups"] = []string{"3A"}
	w = org.DoRequest("POST", fmt.Sprintf("/tracking/organizations/%s/members", MainSchoolID), body, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("invite supervisor: %d %s", w.Code, w.Body.String())
	}
	userID := ParseResponse(t, w.Body.Bytes())["user_id"].(string)
	member := memberRow(t, org, userID)
	if assert.NotNil(t, member) {
		assert.Equal(t, "supervisor", member.Role)
		assert.Equal(t, []string{"3A"}, member.Groups)
	}

	w = org.DoRequest("PUT", fmt.Sprintf("/tracking/organizations/%s/members/%s", MainSchoolID, userID),
		map[string]interface{}{"role": "viewer", "groups": []string{"3A"}}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	if member = memberRow(t, org, userID); assert.NotNil(t, member) {
		assert.Equal(t, "viewer", member.Role)
		assert.Nil(t, member.Groups, "only a supervisor carries groups")
	}

	w = org.DoRequest("DELETE", fmt.Sprintf("/tracking/organizations/%s/members/%s", MainSchoolID, userID), nil, map[string]string{})
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.Nil(t, memberRow(t, org, userID))
}

// TestOrganizationSelf_ViewerReadsOnly - a viewer reads its organization and members but every write
// is refused 403 tracking.organization.scope-denied, a rider's edit included.
func TestOrganizationSelf_ViewerReadsOnly(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	riderID := CreateTestRider(t, staff)
	setOrgUserMembership(t, staff, "viewer", nil)
	org := SetupOrganizationTest(t)
	defer org.Close()

	var own models.Organization
	decodeBody(t, org.DoRequest("GET", "/tracking/organizations/"+MainSchoolID, nil, map[string]string{}), &own)
	if assert.NotNil(t, own.MyRole) {
		assert.Equal(t, "viewer", *own.MyRole)
	}
	assert.Equal(t, http.StatusOK, org.DoRequest("GET", "/tracking/organizations/"+MainSchoolID+"/members", nil, map[string]string{}).Code)

	body := accessPerson(newAccessEmail())
	body["role"] = "viewer"
	w := org.DoRequest("POST", fmt.Sprintf("/tracking/organizations/%s/members", MainSchoolID), body, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.organization.scope-denied")

	w = org.DoRequest("PATCH", "/tracking/organizations/"+MainSchoolID, map[string]interface{}{"name": own.Name}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	w = org.DoRequest("PATCH", "/tracking/riders/"+riderID, map[string]interface{}{"first_name": "Renamed"}, map[string]string{})
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

// TestOrganizationSelf_SupervisorGroups - a supervisor limited to one group lists only that group's
// riders and the trips carrying them; a rider of another group is not found.
func TestOrganizationSelf_SupervisorGroups(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	group, other := "G-"+uuid.NewString()[:6], "G-"+uuid.NewString()[:6]
	ownRider, ownTrip, _ := groupRiding(t, staff, group)
	otherRider, otherTrip, _ := groupRiding(t, staff, other)
	setOrgUserMembership(t, staff, "supervisor", []string{group})
	org := SetupOrganizationTest(t)
	defer org.Close()

	riders := ListRiders(t, org, "?page_size=100")
	ids := map[string]bool{}
	for _, r := range riders.Riders {
		ids[r.ID.String()] = true
		if assert.NotNil(t, r.GroupLabel, r.ID.String()) {
			assert.Equal(t, group, *r.GroupLabel)
		}
	}
	assert.True(t, ids[ownRider])
	assert.False(t, ids[otherRider])
	assert.Equal(t, http.StatusNotFound, org.DoRequest("GET", "/tracking/riders/"+otherRider, nil, map[string]string{}).Code)
	assert.Equal(t, http.StatusOK, org.DoRequest("GET", "/tracking/riders/"+ownRider, nil, map[string]string{}).Code)

	var board models.ListTripsResponse
	decodeBody(t, org.DoRequest("GET", "/tracking/trips?status=in_progress&page_size=100", nil, map[string]string{}), &board)
	trips := map[string]bool{}
	for _, trip := range board.Trips {
		trips[trip.ID.String()] = true
	}
	assert.True(t, trips[ownTrip], "the trip carrying its group")
	assert.False(t, trips[otherTrip], "a trip carrying only another group")
}

// memberRow is the member userID of MainSchoolID as helper reads it, nil when it is not one.
func memberRow(t *testing.T, helper *testhelpers.ApiTestHelper, userID string) *models.OrganizationMember {
	t.Helper()
	var members []models.OrganizationMember
	decodeBody(t, helper.DoRequest("GET", "/tracking/organizations/"+MainSchoolID+"/members", nil, map[string]string{}), &members)
	for i := range members {
		if members[i].UserID.String() == userID {
			return &members[i]
		}
	}
	return nil
}

// TestOrganizationSelf_ViewerLiveMap - a viewer subscribes its organization's fleet channel and gets
// its bus's position there; the tenant's fleet and another organization's channel are forbidden; its
// fleet snapshot lists only the buses carrying its riders.
func TestOrganizationSelf_ViewerLiveMap(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	if err := staff.Valkey().Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("Valkey unreachable: %v", err)
	}
	_, _, vehicleID := groupRiding(t, staff, "G-"+uuid.NewString()[:6])
	foreign := createOrganization(t, staff)
	_, _, foreignVehicle := groupRiding(t, staff, "G-"+uuid.NewString()[:6])
	foreignRider := createRider(t, staff, nil)
	MoveRiderToOrganization(t, staff, foreignRider, foreign)
	setOrgUserMembership(t, staff, "viewer", nil)
	server := httptest.NewServer(staff.Engine())
	defer server.Close()
	org := SetupOrganizationTest(t)
	defer org.Close()

	var fleet models.LiveFleetResponse
	decodeBody(t, org.DoRequest("GET", "/tracking/live/fleet", nil, map[string]string{}), &fleet)
	listed := map[string]bool{}
	for _, v := range fleet.Vehicles {
		listed[v.VehicleID.String()] = true
	}
	assert.True(t, listed[vehicleID], "the bus carrying its riders")
	assert.True(t, listed[foreignVehicle], "both buses carry the school's riders")
	assert.Equal(t, http.StatusForbidden, org.DoRequest("GET", "/tracking/live/fleet?organization_id="+foreign, nil, map[string]string{}).Code)

	socket := dialWS(t, server, org)
	defer socket.conn.CloseNow()
	own := trackingJobs.FleetOrgChannel(TestTenantID, MainSchoolID)
	socket.send(t, "subscribe", own)
	for _, channel := range []string{trackingJobs.FleetChannel(TestTenantID), trackingJobs.FleetOrgChannel(TestTenantID, foreign)} {
		socket.send(t, "subscribe", channel)
		refusal := socket.waitFrame(channel, "error", 2*time.Second)
		if assert.NotNil(t, refusal, channel) {
			assert.Equal(t, "forbidden", refusal["code"], channel)
		}
	}

	consumer := startTestConsumer(t, staff, time.Minute)
	defer consumer.Shutdown()
	xadd(t, staff, models.StoredPoint{VehicleID: uuid.MustParse(vehicleID), RecordedAt: time.Now().UTC(), Lat: -17.3930, Lng: -66.1560})
	assert.NotNil(t, socket.waitFrame(own, "position", 5*time.Second), "the position reaches the organization's channel")
}

// TestOrganizationSelf_SupervisorLiveMap - a supervisor with groups may not use the organization's
// fleet channel (it carries other groups' buses) but watches the trip carrying its group; its fleet
// snapshot lists only that bus.
func TestOrganizationSelf_SupervisorLiveMap(t *testing.T) {
	staff := SetupTrackingTest(t)
	defer staff.Close()
	group := "G-" + uuid.NewString()[:6]
	_, ownTrip, ownVehicle := groupRiding(t, staff, group)
	_, _, otherVehicle := groupRiding(t, staff, "G-"+uuid.NewString()[:6])
	setOrgUserMembership(t, staff, "supervisor", []string{group})
	server := httptest.NewServer(staff.Engine())
	defer server.Close()
	org := SetupOrganizationTest(t)
	defer org.Close()

	var fleet models.LiveFleetResponse
	decodeBody(t, org.DoRequest("GET", "/tracking/live/fleet", nil, map[string]string{}), &fleet)
	listed := map[string]bool{}
	for _, v := range fleet.Vehicles {
		listed[v.VehicleID.String()] = true
	}
	assert.True(t, listed[ownVehicle])
	assert.False(t, listed[otherVehicle])

	// Asked of the SP itself: the gateway keeps an answer per (user, channel) for five minutes, and the
	// viewer test before this one has the same user on the same channel.
	var allowed bool
	if err := staff.DB().QueryRow(context.Background(), `
		SELECT tracking.sp_can_subscribe(p_tenant_id := $1, p_user_id := u.id, p_guardian := false, p_channel := $2, p_organization := true)
		FROM auth.users u WHERE u.email = 'orguser@test.local'`,
		TestTenantID, trackingJobs.FleetOrgChannel(TestTenantID, MainSchoolID)).Scan(&allowed); err != nil {
		t.Fatalf("can subscribe: %v", err)
	}
	assert.False(t, allowed, "a supervisor with groups may not use the organization's fleet channel")

	socket := dialWS(t, server, org)
	defer socket.conn.CloseNow()
	trip := trackingJobs.TripChannel(ownTrip)
	socket.send(t, "subscribe", trip)
	assert.Nil(t, socket.waitFrame(trip, "error", time.Second), "its group's trip is allowed")
}
