//go:build integration
// +build integration

package tracking_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fleet setup from the vehicle form (TRACK-047): a company needs only its name, a driver needs no
// licence, and a driver's app access is granted in the request that creates the driver.

// TestCreateCompanyNameOnly - phone and address are optional (D2).
func TestCreateCompanyNameOnly(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	name := "Quick Company " + uuid.New().String()[:8]
	w := helper.DoRequest("POST", "/tracking/companies", map[string]interface{}{"name": name}, map[string]string{})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	company := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, name, company["name"])
	assert.Nil(t, company["phone"])
	assert.Nil(t, company["address"])
}

// TestCreateDriverWithAccount - one POST creates the driver, links a new account at the driver level
// and answers the invitation; two drivers may both have no licence yet (D3, D4).
func TestCreateDriverWithAccount(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	email := newAccessEmail()
	body := map[string]interface{}{
		"company_id": MainCompanyID,
		"first_name": "Juan",
		"last_name":  "Mamani " + uuid.New().String()[:6],
		"phone":      "+59170000001",
		"account":    map[string]interface{}{"email": email},
	}
	w := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	driver := ParseResponse(t, w.Body.Bytes())
	assert.Equal(t, email, driver["user_email"])
	assert.Nil(t, driver["license_number"])

	account, ok := driver["account"].(map[string]interface{})
	require.True(t, ok, "the 201 carries the account")
	assert.Equal(t, email, account["email"])
	assert.Equal(t, "new", account["status"])
	assert.NotEmpty(t, account["notice"])
	userID := account["user_id"].(string)
	assert.Equal(t, userID, driver["user_id"])

	assert.Equal(t, 1, CountTenantMemberships(t, helper, userID))
	role, active := TenantMembership(t, helper, userID)
	assert.Equal(t, "driver", role)
	assert.True(t, active)
	assert.Equal(t, 1, CountUsersByEmail(t, helper, email))

	plain := ValidDriverBody()
	delete(plain, "license_number")
	w = helper.DoRequest("POST", "/tracking/drivers", plain, map[string]string{})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := ParseResponse(t, w.Body.Bytes())
	assert.Nil(t, created["account"])
	assert.Nil(t, created["license_number"])
}

// TestCreateDriverRefusedAccount - an email with web access is refused with 409 and no driver is left.
func TestCreateDriverRefusedAccount(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()

	surname := "Refused " + uuid.New().String()[:8]
	body := map[string]interface{}{
		"company_id": MainCompanyID,
		"first_name": "Ana",
		"last_name":  surname,
		"account":    map[string]interface{}{"email": "admin@test.local"},
	}
	w := helper.DoRequest("POST", "/tracking/drivers", body, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "tracking.access.web-account")

	w = helper.DoRequest("GET", "/tracking/drivers?search="+url.QueryEscape(surname), nil, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, float64(0), ParseResponse(t, w.Body.Bytes())["total_count"])
}
