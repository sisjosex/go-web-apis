package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	coreTestHelpers "josex/web/modules/core/testhelpers"
	"josex/web/modules/tenancy/middleware"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockTenantService implements interfaces.TenantService for unit tests.
type mockTenantService struct {
	accessInfo   *models.TenantAccessInfo
	err          error
	tenantBySlug *models.Tenant
	slugErr      error
}

func (m *mockTenantService) GetTenantBySlug(_ context.Context, _ string) (*models.Tenant, error) {
	return m.tenantBySlug, m.slugErr
}
func (m *mockTenantService) VerifyUserTenantAccess(_ context.Context, _ uuid.UUID, _ string) (*models.TenantAccessInfo, error) {
	return m.accessInfo, m.err
}
func (m *mockTenantService) CreateTenant(_ context.Context, _ *models.CreateTenantDto, _ uuid.UUID) (*models.TenantDetailResponse, error) {
	return nil, nil
}
func (m *mockTenantService) GetUserTenants(_ context.Context, _ uuid.UUID) ([]*models.UserTenantResponse, error) {
	return nil, nil
}
func (m *mockTenantService) UpdateTenant(_ context.Context, _ uuid.UUID, _ *models.UpdateTenantDto) (*models.TenantDetailResponse, error) {
	return nil, nil
}
func (m *mockTenantService) AddUserToTenant(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ *models.AddUserToTenantDto) error {
	return nil
}
func (m *mockTenantService) RemoveUserFromTenant(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockTenantService) CountUserOwnedTenants(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}
func (m *mockTenantService) RunTenantMigrations(_ context.Context, _ string) error {
	return nil
}
func (m *mockTenantService) GetTenantEnabledModuleCodes(_ context.Context, _ uuid.UUID) (map[string]bool, error) {
	return make(map[string]bool), nil
}
func (m *mockTenantService) UpdateUserRole(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ uuid.UUID, _ string) error {
	return nil
}

func init() {
	// Load .env.test so JWT_SECRET_KEY and TENANCY_ENABLED are available
	coreTestHelpers.InitTestEnvironment()
	gin.SetMode(gin.TestMode)
	config.GetConfig()
}

// runMiddleware executes TenantMiddleware in a test Gin context and returns
// the captured request context and HTTP status code.
func runMiddleware(tenantSlug string, userID uuid.UUID, svc *mockTenantService) (context.Context, int) {
	var capturedCtx context.Context
	statusCode := http.StatusOK

	router := gin.New()
	router.GET("/:tenant_slug/test", middleware.TenantMiddleware(svc), func(c *gin.Context) {
		// Capture the request context after middleware ran
		capturedCtx = c.Request.Context()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/"+tenantSlug+"/test", nil)

	// Simulate what AuthMiddleware would have set
	w := httptest.NewRecorder()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Next()
	})

	// Build an engine that sets user_id before TenantMiddleware
	eng := gin.New()
	eng.GET("/:tenant_slug/test",
		func(c *gin.Context) {
			c.Set("user_id", userID.String())
			c.Next()
		},
		middleware.TenantMiddleware(svc),
		func(c *gin.Context) {
			capturedCtx = c.Request.Context()
			c.Status(http.StatusOK)
		},
	)

	eng.ServeHTTP(w, req)
	statusCode = w.Code
	return capturedCtx, statusCode
}

// TestTenantMiddleware_InjectsDatabaseURLIntoContext verifies that when a
// tenant has a database_url, the middleware puts it into the request context
// under TenantDatabaseURLKey so DatabaseService.resolvePool picks it up.
func TestTenantMiddleware_InjectsDatabaseURLIntoContext(t *testing.T) {
	tenantURL := "postgres://tenant:pass@db.example.com:5432/tenant_db?sslmode=disable"
	userID := uuid.New()
	tenantID := uuid.New()

	svc := &mockTenantService{
		accessInfo: &models.TenantAccessInfo{
			TenantID:    tenantID,
			Slug:        "acme",
			DatabaseURL: &tenantURL,
			IsActive:    true,
			IsSuspended: false,
			UserRole:    "admin",
		},
	}

	capturedCtx, status := runMiddleware("acme", userID, svc)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if capturedCtx == nil {
		t.Fatal("context was not captured (handler never ran)")
	}

	injectedURL, ok := capturedCtx.Value(coreServices.TenantDatabaseURLKey).(string)
	if !ok || injectedURL == "" {
		t.Fatal("TenantDatabaseURLKey was not injected into request context")
	}

	if injectedURL != tenantURL {
		t.Errorf("expected URL %q, got %q", tenantURL, injectedURL)
	}
}

// TestTenantMiddleware_NoURLInContext_WhenTenantHasNoDatabaseURL verifies
// that when the tenant record has no database_url, no key is injected —
// queries will fall through to the primary (platform) pool.
func TestTenantMiddleware_NoURLInContext_WhenTenantHasNoDatabaseURL(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	svc := &mockTenantService{
		accessInfo: &models.TenantAccessInfo{
			TenantID:    tenantID,
			Slug:        "shared-tenant",
			DatabaseURL: nil, // no dedicated DB
			IsActive:    true,
			IsSuspended: false,
			UserRole:    "member",
		},
	}

	capturedCtx, status := runMiddleware("shared-tenant", userID, svc)

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	injectedURL, _ := capturedCtx.Value(coreServices.TenantDatabaseURLKey).(string)
	if injectedURL != "" {
		t.Errorf("expected no URL in context, but got %q", injectedURL)
	}
}

// TestTenantMiddleware_Returns403_WhenTenantInactive verifies the middleware
// blocks access for inactive tenants before touching any DB pool.
func TestTenantMiddleware_Returns403_WhenTenantInactive(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	tenantURL := "postgres://x:x@db/tenant"

	svc := &mockTenantService{
		accessInfo: &models.TenantAccessInfo{
			TenantID:    tenantID,
			Slug:        "inactive-co",
			DatabaseURL: &tenantURL,
			IsActive:    false, // tenant is disabled
			IsSuspended: false,
			UserRole:    "admin",
		},
	}

	_, status := runMiddleware("inactive-co", userID, svc)

	if status != http.StatusForbidden {
		t.Errorf("expected 403 for inactive tenant, got %d", status)
	}
}

// runMiddlewareFromHeader exercises TenantMiddlewareFromHeader with the
// X-Tenant-Slug header and a pre-set system_role in context.
func runMiddlewareFromHeader(tenantSlug string, userID uuid.UUID, systemRole string, svc *mockTenantService) (context.Context, int) {
	var capturedCtx context.Context

	eng := gin.New()
	eng.GET("/test",
		func(c *gin.Context) {
			c.Set("user_id", userID.String())
			c.Set("system_role", systemRole)
			c.Next()
		},
		middleware.TenantMiddlewareFromHeader(svc),
		func(c *gin.Context) {
			capturedCtx = c.Request.Context()
			c.Status(http.StatusOK)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if tenantSlug != "" {
		req.Header.Set("X-Tenant-Slug", tenantSlug)
	}

	w := httptest.NewRecorder()
	eng.ServeHTTP(w, req)
	return capturedCtx, w.Code
}

// TestTenantMiddlewareFromHeader_MissingHeader_Returns400 verifies that all roles
// get 400 when X-Tenant-Slug is absent — no bypass exists anymore.
func TestTenantMiddlewareFromHeader_MissingHeader_Returns400(t *testing.T) {
	svc := &mockTenantService{}
	userID := uuid.New()

	for _, role := range []string{"user", "admin", "super_admin"} {
		_, status := runMiddlewareFromHeader("", userID, role, svc)
		if status != http.StatusBadRequest {
			t.Errorf("role=%q: expected 400 when header missing, got %d", role, status)
		}
	}
}

// TestTenantMiddlewareFromHeader_SuperAdminBypassesMembership verifies that
// super_admin can access any tenant via GetTenantBySlug without being in tenant_users.
func TestTenantMiddlewareFromHeader_SuperAdminBypassesMembership(t *testing.T) {
	tenantID := uuid.New()
	tenantURL := "postgres://tenant:pass@db.example.com:5432/tenant_db"

	svc := &mockTenantService{
		tenantBySlug: &models.Tenant{
			ID:          tenantID,
			Slug:        "any-tenant",
			Name:        "Any Tenant",
			DatabaseURL: &tenantURL,
			SchemaName:  "public",
			IsActive:    true,
			IsSuspended: false,
		},
		// accessInfo is nil — super_admin path must NOT call VerifyUserTenantAccess
	}

	capturedCtx, status := runMiddlewareFromHeader("any-tenant", uuid.New(), "super_admin", svc)

	if status != http.StatusOK {
		t.Fatalf("expected 200 for super_admin, got %d", status)
	}

	injectedURL, ok := capturedCtx.Value(coreServices.TenantDatabaseURLKey).(string)
	if !ok || injectedURL != tenantURL {
		t.Errorf("expected tenant DB URL in context, got %q", injectedURL)
	}
}

// TestTenantMiddlewareFromHeader_RegularUser_NoMembership_Returns403 verifies
// that a regular user with no membership in the tenant gets 403.
func TestTenantMiddlewareFromHeader_RegularUser_NoMembership_Returns403(t *testing.T) {
	svc := &mockTenantService{
		accessInfo: nil,
		err:        fmt.Errorf("tenant.user.unauthorized"),
	}

	_, status := runMiddlewareFromHeader("some-tenant", uuid.New(), "user", svc)

	if status != http.StatusForbidden {
		t.Errorf("expected 403 for non-member user, got %d", status)
	}
}

// TestTenantMiddleware_Returns403_WhenTenantSuspended verifies suspended
// tenants are blocked even if they have a valid database_url.
func TestTenantMiddleware_Returns403_WhenTenantSuspended(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	tenantURL := "postgres://x:x@db/tenant"

	svc := &mockTenantService{
		accessInfo: &models.TenantAccessInfo{
			TenantID:    tenantID,
			Slug:        "suspended-co",
			DatabaseURL: &tenantURL,
			IsActive:    true,
			IsSuspended: true, // suspended
			UserRole:    "admin",
		},
	}

	_, status := runMiddleware("suspended-co", userID, svc)

	if status != http.StatusForbidden {
		t.Errorf("expected 403 for suspended tenant, got %d", status)
	}
}

// TestTenantMiddlewareFromHeader_PortalUser_Returns403 verifies that a mobile-only guardian
// (TRACK-015 D2) is refused on every web tenant route, with the code the app reads to show
// "this account is mobile-only".
func TestTenantMiddlewareFromHeader_PortalUser_Returns403(t *testing.T) {
	svc := &mockTenantService{
		accessInfo: &models.TenantAccessInfo{
			TenantID:     uuid.New(),
			Slug:         "some-tenant",
			Name:         "Some Tenant",
			SchemaName:   "public",
			IsActive:     true,
			IsSuspended:  false,
			UserRole:     models.RolePortal,
			UserIsActive: true,
			Permissions:  make(map[string]bool),
		},
	}

	_, status := runMiddlewareFromHeader("some-tenant", uuid.New(), "user", svc)

	if status != http.StatusForbidden {
		t.Errorf("expected 403 for a portal account on the web, got %d", status)
	}
}

// runDenyTenantRole executes DenyTenantRole behind the context TenantMiddleware would have set.
func runDenyTenantRole(tenantRole, systemRole string, denied ...string) int {
	eng := gin.New()
	eng.GET("/test",
		func(c *gin.Context) {
			if tenantRole != "" {
				c.Set("tenant_user_role", tenantRole)
			}
			c.Set("system_role", systemRole)
			c.Next()
		},
		middleware.DenyTenantRole(denied...),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	w := httptest.NewRecorder()
	eng.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
	return w.Code
}

func TestDenyTenantRole(t *testing.T) {
	cases := []struct {
		name       string
		tenantRole string
		systemRole string
		want       int
	}{
		{"denied level is refused", models.RoleOrganization, "user", http.StatusForbidden},
		{"another level passes", models.RoleMember, "user", http.StatusOK},
		{"super_admin passes", models.RoleOrganization, "super_admin", http.StatusOK},
		{"no level at all is unauthorized", "", "user", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		if got := runDenyTenantRole(tc.tenantRole, tc.systemRole, models.RoleOrganization); got != tc.want {
			t.Errorf("%s: expected %d, got %d", tc.name, tc.want, got)
		}
	}
}
