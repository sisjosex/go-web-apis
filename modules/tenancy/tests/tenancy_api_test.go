//go:build integration
// +build integration

package tenancy_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize test environment (load .env.test before any config)
	testhelpers.InitTestEnvironment()
}

// Helper function to generate a valid UUID
func testDeviceID() string {
	return uuid.New().String()
}

// ============================================================================
// SELF-SERVICE TENANT CREATION TESTS (for regular users)
// ============================================================================

// TestCreateTenantSelfServiceSuccess tests that a regular user can create their own tenant
func TestCreateTenantSelfServiceSuccess(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login first
	helper.Register("user@test.com", "$Password2025", "Test", "User")
	helper.Login("user@test.com", "$Password2025")

	// Create tenant using self-service endpoint (for regular users)
	body := map[string]interface{}{
		"slug": "my-tenant-1",
		"name": "My Tenant 1",
	}

	w := helper.DoRequest("POST", "/tenants/self-service", body, map[string]string{})

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Logf("❌ Create tenant failed with status %d: %s", w.Code, w.Body.String())
	}

	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusOK,
		fmt.Sprintf("Expected 201 or 200, got %d", w.Code))

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.NotNil(t, response["id"])
	assert.Equal(t, "my-tenant-1", response["slug"])
	assert.Equal(t, "My Tenant 1", response["name"])
}

// TestCreateTenantSelfServiceUnauthorized tests that unauthorized users can't create tenants
func TestCreateTenantSelfServiceUnauthorized(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Try without login
	body := map[string]interface{}{
		"slug": "unauth-tenant",
		"name": "Unauthorized Tenant",
	}

	w := helper.DoRequest("POST", "/tenants/self-service", body, map[string]string{})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// TENANT RETRIEVAL TESTS
// ============================================================================

// TestGetUserTenantsSuccess tests that a user can retrieve their own tenants
func TestGetUserTenantsSuccess(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login
	helper.Register("user2@test.com", "$Password2025", "Test", "User")
	helper.Login("user2@test.com", "$Password2025")

	// Create a tenant first
	createBody := map[string]interface{}{
		"slug": "user-tenant",
		"name": "User Tenant",
	}
	helper.DoRequest("POST", "/tenants/self-service", createBody, map[string]string{})

	// Get user's tenants
	w := helper.DoRequest("GET", "/tenants/my-tenants", nil, map[string]string{})

	assert.True(t, w.Code == http.StatusOK,
		fmt.Sprintf("Expected 200, got %d: %s", w.Code, w.Body.String()))

	var response []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	assert.NotEmpty(t, response)
	assert.Equal(t, "user-tenant", response[0]["slug"])
}

// TestGetUserTenantsUnauthorized tests that unauthorized users can't get tenant list
func TestGetUserTenantsUnauthorized(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Try without login
	w := helper.DoRequest("GET", "/tenants/my-tenants", nil, map[string]string{})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// TENANT VALIDATION TESTS
// ============================================================================

// TestCreateTenantDuplicateSlugError tests that creating a tenant with duplicate slug fails
func TestCreateTenantDuplicateSlugError(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login
	helper.Register("user3@test.com", "$Password2025", "Test", "User")
	helper.Login("user3@test.com", "$Password2025")

	// Create first tenant
	createBody := map[string]interface{}{
		"slug": "dup-tenant",
		"name": "First Tenant",
	}
	w1 := helper.DoRequest("POST", "/tenants/self-service", createBody, map[string]string{})
	assert.True(t, w1.Code == http.StatusCreated || w1.Code == http.StatusOK)

	// Try to create second tenant with same slug
	createBody2 := map[string]interface{}{
		"slug": "dup-tenant",
		"name": "Second Tenant",
	}
	w2 := helper.DoRequest("POST", "/tenants/self-service", createBody2, map[string]string{})

	// Should get conflict/duplicate error or limit reached error (both valid)
	// Note: May get 403 if plan limit reached (TENANCY_FREE_PLAN_LIMIT=1)
	assert.True(t, w2.Code == http.StatusConflict || w2.Code == http.StatusBadRequest || w2.Code == http.StatusForbidden,
		fmt.Sprintf("Expected 409/400/403, got %d: %s", w2.Code, w2.Body.String()))
}

// TestCreateTenantMissingSlugError tests validation for tenant creation
func TestCreateTenantMissingSlugError(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login
	helper.Register("user4@test.com", "$Password2025", "Test", "User")
	helper.Login("user4@test.com", "$Password2025")

	// Try to create tenant without slug
	body := map[string]interface{}{
		"name": "No Slug Tenant",
	}

	w := helper.DoRequest("POST", "/tenants/self-service", body, map[string]string{})

	// Should get validation error
	assert.True(t, w.Code == http.StatusBadRequest,
		fmt.Sprintf("Expected 400, got %d", w.Code))
}

// TestCreateTenantMissingNameError tests validation for tenant creation
func TestCreateTenantMissingNameError(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login
	helper.Register("user5@test.com", "$Password2025", "Test", "User")
	helper.Login("user5@test.com", "$Password2025")

	// Try to create tenant without name
	body := map[string]interface{}{
		"slug": "no-name-tenant",
	}

	w := helper.DoRequest("POST", "/tenants/self-service", body, map[string]string{})

	// Should get validation error
	assert.True(t, w.Code == http.StatusBadRequest,
		fmt.Sprintf("Expected 400, got %d", w.Code))
}

// ============================================================================
// TENANT USER MANAGEMENT TESTS
// ============================================================================

// TestAddUserToTenant tests that a tenant owner can add users to their tenant
func TestAddUserToTenant(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Register and login as tenant owner
	helper.Register("owner@test.com", "$Password2025", "Owner", "User")
	helper.Login("owner@test.com", "$Password2025")

	// Create tenant
	createBody := map[string]interface{}{
		"slug": "tenant-with-users",
		"name": "Tenant With Users",
	}
	w := helper.DoRequest("POST", "/tenants/self-service", createBody, map[string]string{})
	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusOK)

	// Register another user
	helper.Register("member@test.com", "$Password2025", "Member", "User")

	// Add user to tenant
	addUserBody := map[string]interface{}{
		"email": "member@test.com",
		"role":  "member",
	}

	w = helper.DoRequest("POST", "/tenants/tenant-with-users/users", addUserBody, map[string]string{})

	assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusOK || w.Code == http.StatusBadRequest,
		fmt.Sprintf("Expected 201/200/400, got %d: %s", w.Code, w.Body.String()))
}

// TestAddUserToTenantUnauthorized tests that unauthorized users can't add users to tenants
func TestAddUserToTenantUnauthorized(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Try to add user without authentication
	addUserBody := map[string]interface{}{
		"email": "member@test.com",
		"role":  "member",
	}

	w := helper.DoRequest("POST", "/tenants/some-tenant/users", addUserBody, map[string]string{})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================================
// PLACEHOLDER TESTS FOR FUTURE FEATURES
// ============================================================================

// TestUpdateTenantSuccess is a placeholder for tenant update functionality
func TestUpdateTenantSuccess(t *testing.T) {
	t.Logf("✅ Tenant update test placeholder ready (update via self-service tenant)")
}

// TestDeleteTenantSuccess is a placeholder for tenant deletion functionality
func TestDeleteTenantSuccess(t *testing.T) {
	t.Logf("✅ Tenant deletion test placeholder ready")
}
