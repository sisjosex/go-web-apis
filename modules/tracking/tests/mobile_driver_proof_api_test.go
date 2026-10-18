//go:build integration
// +build integration

package tracking_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"josex/web/modules/core/services/storage/storagetest"
	"josex/web/modules/core/testhelpers"
	"josex/web/modules/tracking/models"
)

// ============================================================================
// DROP-OFF PROOF AND LATE UNDO (MOBILE-032)
// ============================================================================

// webpBytes is n bytes that sniff as image/webp: the RIFF container's WEBPVP8 header, then padding.
func webpBytes(n int) []byte {
	b := make([]byte, n)
	copy(b, "RIFF\x00\x00\x00\x00WEBPVP8 ")
	return b
}

// dropoffOf is the trip's first drop-off task.
func dropoffOf(t *testing.T, trip models.TripDetail) models.TripTask {
	t.Helper()
	for _, stop := range trip.Stops {
		for _, task := range stop.Tasks {
			if task.Kind == "dropoff" {
				return task
			}
		}
	}
	t.Fatalf("trip %s has no drop-off", trip.ID)
	return models.TripTask{}
}

// uploadProof takes a ticket as the driver and PUTs content to it, answering the photo_key.
func uploadProof(t *testing.T, driver *testhelpers.ApiTestHelper, content []byte) string {
	t.Helper()
	w := driver.DoRequest("POST", "/mobile/driver/proofs", models.DriverProofDto{Size: int64(len(content))}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("proof ticket: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var ticket models.DriverProofTicket
	decodeBody(t, w, &ticket)
	PutToTicket(t, ticket.UploadURL, ticket.Headers, content)
	return ticket.Key
}

// TestDriverProof_DropOffWithPhoto - ticket → PUT → task.done naming the photo_key: the task's proof
// names trip-proofs/<tenant>/<task>.webp, the trip detail says has_photo, the operator's link serves
// image/webp, and a guardian reads it only once the rider is theirs.
func TestDriverProof_DropOffWithPhoto(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	dropoff := dropoffOf(t, trip)
	final := "trip-proofs/" + TestTenantID + "/" + dropoff.ID.String() + ".webp"
	t.Cleanup(func() { _ = storagetest.Documents(t).Delete(context.Background(), final) })

	photoKey := uploadProof(t, driver, webpBytes(2048))
	done := taskOp(models.DriverOpTaskDone, dropoff.ID, time.Now().UTC().Add(-time.Minute))
	done.Args.Proof = map[string]any{"photo_key": photoKey}
	results := sync(t, driver, done)

	if assert.Len(t, results, 1) {
		assert.Equal(t, models.DriverOpApplied, results[0].Status, "%v", results[0].Code)
	}
	assert.JSONEq(t, `{"photo": "`+final+`"}`, TaskProof(t, helper, dropoff.ID))
	assert.True(t, dropoffOf(t, tripRequest(t, helper, "GET", "/tracking/trips/"+trip.ID.String(), nil, http.StatusOK)).HasPhoto)

	w := helper.DoRequest("GET", "/tracking/trip-stop-tasks/"+dropoff.ID.String()+"/proof", nil, map[string]string{})
	if assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		var proof models.TripTaskProof
		decodeBody(t, w, &proof)
		res, err := http.Get(proof.URL)
		if assert.NoError(t, err) {
			_ = res.Body.Close()
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, "image/webp", res.Header.Get("Content-Type"))
		}
	}

	guardian := SetupPortalTest(t)
	defer guardian.Close()
	w = guardian.DoRequest("GET", "/tracking/trip-stop-tasks/"+dropoff.ID.String()+"/proof", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, "another family's task: %s", w.Body.String())
	LinkPortalGuardian(t, helper, dropoff.SubjectID.String())
	w = guardian.DoRequest("GET", "/tracking/trip-stop-tasks/"+dropoff.ID.String()+"/proof", nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = guardian.DoRequest("GET", "/tracking/riders/"+dropoff.SubjectID.String()+"/status", nil, map[string]string{})
	if assert.Equal(t, http.StatusOK, w.Code, w.Body.String()) {
		var status models.RiderStatusResponse
		decodeBody(t, w, &status)
		assert.True(t, status.LastEventHasPhoto)
		if assert.NotNil(t, status.LastEventTaskID) {
			assert.Equal(t, dropoff.ID, *status.LastEventTaskID)
		}
	}
}

// TestDriverProof_BadUploadNeverRefusesTheDropOff - a photo_key never uploaded, or another tenant's,
// still applies the drop-off, with { photo_failed }; a ticket over 300 KB is a 400.
func TestDriverProof_BadUploadNeverRefusesTheDropOff(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	dropoff := dropoffOf(t, trip)

	done := taskOp(models.DriverOpTaskDone, dropoff.ID, time.Now().UTC().Add(-time.Minute))
	done.Args.Proof = map[string]any{"photo_key": "pending/" + uuid.NewString() + "/" + uuid.NewString() + ".webp"}
	results := sync(t, driver, done)

	if assert.Len(t, results, 1) {
		assert.Equal(t, models.DriverOpApplied, results[0].Status, "%v", results[0].Code)
	}
	assert.JSONEq(t, `{"photo_failed": true}`, TaskProof(t, helper, dropoff.ID))
	w := helper.DoRequest("GET", "/tracking/trip-stop-tasks/"+dropoff.ID.String()+"/proof", nil, map[string]string{})
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	w = driver.DoRequest("POST", "/mobile/driver/proofs", models.DriverProofDto{Size: 307201}, map[string]string{})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestDriverSync_Reset - a task.reset 2 min after a done returns the task to pending with one reset
// event; one 16 min after its mark is rejected reset-expired; an undone no_show gives the rider back
// the drop-off it cancelled (D2).
func TestDriverSync_Reset(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	trip := driverTrip(t, helper, 1, true)
	driver := SetupDriverTest(t)
	defer driver.Close()
	pickup := trip.Stops[0].Tasks[0]
	now := time.Now().UTC()

	results := sync(t, driver,
		taskOp(models.DriverOpTaskDone, pickup.ID, now.Add(-2*time.Minute)),
		taskOp(models.DriverOpTaskReset, pickup.ID, now))
	assert.Equal(t, []string{models.DriverOpApplied, models.DriverOpApplied}, statusesOf(results), "%v", results)
	assert.ElementsMatch(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, trip.ID.String()))
	assert.Equal(t, 1, TripEventCount(t, helper, trip.ID, "reset"))
	assert.Equal(t, "", TaskProof(t, helper, pickup.ID))

	results = sync(t, driver,
		taskOp(models.DriverOpTaskNoShow, pickup.ID, now.Add(-time.Minute)),
		taskOp(models.DriverOpTaskReset, pickup.ID, now))
	assert.Equal(t, []string{models.DriverOpApplied, models.DriverOpApplied}, statusesOf(results), "%v", results)
	assert.ElementsMatch(t, []string{"dropoff:pending", "pickup:pending"}, taskStatuses(t, helper, trip.ID.String()))

	sync(t, driver, taskOp(models.DriverOpTaskDone, pickup.ID, now.Add(-16*time.Minute)))
	results = sync(t, driver, taskOp(models.DriverOpTaskReset, pickup.ID, time.Now().UTC()))
	if assert.Len(t, results, 1) {
		assert.Equal(t, models.DriverOpRejected, results[0].Status)
		assert.Equal(t, "tracking.trip.task.reset-expired", *results[0].Code)
	}
	assert.ElementsMatch(t, []string{"dropoff:pending", "pickup:done"}, taskStatuses(t, helper, trip.ID.String()))
}
