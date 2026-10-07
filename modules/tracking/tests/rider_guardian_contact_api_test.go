//go:build integration
// +build integration

package tracking_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAddRiderGuardian_Contact - TRACK-048 D3: the Apoderados tab adds a guardian without an email as
// a contact (no account), as the create does; a contact needs a name and a phone.
func TestAddRiderGuardian_Contact(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	riderID := createAccessRider(t, helper)
	path := fmt.Sprintf("/tracking/riders/%s/guardians", riderID)

	w := helper.DoRequest("POST", path, map[string]interface{}{
		"first_name": "Rosa", "last_name": "Choque", "phone": "+59172222222",
	}, map[string]string{})
	expectStatus(t, w, http.StatusCreated)
	added := ParseResponse(t, w.Body.Bytes())
	assert.Nil(t, added["user_id"], "a contact gets no account")
	assert.Equal(t, "contact", added["status"])

	w = helper.DoRequest("GET", path, nil, map[string]string{})
	expectStatus(t, w, http.StatusOK)
	listed := ParseListResponse(t, w.Body.Bytes())
	if assert.Len(t, listed, 1) {
		assert.Nil(t, listed[0]["user_id"])
		assert.Equal(t, "Rosa Choque", listed[0]["first_name"])
		assert.Equal(t, "+59172222222", listed[0]["phone"])
	}

	w = helper.DoRequest("POST", path, map[string]interface{}{"first_name": "Sin teléfono"}, map[string]string{})
	expectStatus(t, w, http.StatusBadRequest)
	assert.Equal(t, "tracking.rider.guardian-invalid", ParseResponse(t, w.Body.Bytes())["error"])
}
