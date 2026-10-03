// filepath: modules/core/testhelpers/api_test_helper.go
package testhelpers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/realtime"
	"josex/web/routes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The router and the database service are built once per test binary and shared
// by every test in the package. They used to be rebuilt inside SetupApiTest, so
// each test opened its own pgxpool (DATABASE_POOL_SIZE connections) that nothing
// ever closed — a package with a hundred tests exhausted PostgreSQL's
// max_connections long before it finished. Tests never run in parallel here, so
// one router and one pool serve them all.
var (
	sharedOnce      sync.Once
	sharedEngine    *gin.Engine
	sharedDBService coreServices.DatabaseService
	sharedValkey    coreServices.ValkeyService
)

// sharedTestServer returns the process-wide router and database service,
// initializing them on first use.
func sharedTestServer() (*gin.Engine, coreServices.DatabaseService) {
	sharedOnce.Do(func() {
		// Skip migrations: they already ran via `go run ./cmd/testutil -reset`.
		os.Setenv("SKIP_MIGRATIONS", "true")
		defer os.Unsetenv("SKIP_MIGRATIONS")

		// Silence the repetitive startup logs.
		oldOut := log.Writer()
		log.SetOutput(io.Discard)
		defer log.SetOutput(oldOut)

		// context.Background(), not a per-test context: the pool outlives every
		// individual test and is closed when the test binary exits.
		dbService := coreServices.NewDatabaseService()
		dbService.InitDatabase(context.Background())

		gin.SetMode(gin.TestMode)
		engine := gin.New()

		// REDIS_URL in .env.test points at its own Valkey database, so the jobs
		// tests never read the dev server's queues.
		valkey, err := coreServices.NewValkeyService(config.ModularAppConfig.Core.RedisURL)
		if err != nil {
			panic(err)
		}

		coreServices.LoadAllTranslations([]string{"en", "es"})
		// The WebSocket gateway, as the all role runs it (TRACK-025).
		var hub *realtime.Hub
		if valkey != nil {
			hub = realtime.NewHub(valkey.Client(), config.ModularAppConfig.Tracking.WSQueueMax, nil)
			hub.Start(context.Background()) //nolint:forbidigo // startup: the hub lives as long as the test binary
		}
		routes.SetupRoutes(engine, dbService, valkey, hub)

		sharedEngine = engine
		sharedValkey = valkey
		sharedDBService = dbService
	})

	return sharedEngine, sharedDBService
}

// InitTestEnvironment loads .env.test before any config is initialized
// This should be called in init() of test files
func InitTestEnvironment() {
	// Set ENV_FILE so LoadEnv() knows to load .env.test
	os.Setenv("ENV_FILE", ".env.test")

	envFilePath := findEnvTestFile()
	if envFilePath != "" {
		loadEnvFileIntoOsEnviron(envFilePath)
	}
}

// findEnvTestFile searches for .env.test in parent directories
func findEnvTestFile() string {
	for _, candidate := range []string{
		".env.test",
		"../../.env.test",
		"../../../.env.test",
		"../../../../.env.test",
		"../../../../../.env.test",
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// loadEnvFileIntoOsEnviron loads .env.test into os.Environ()
func loadEnvFileIntoOsEnviron(filepath string) {
	envFile, err := os.Open(filepath)
	if err != nil {
		log.Printf("⚠️  Could not load %s: %v", filepath, err)
		return
	}
	defer envFile.Close()

	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
}

// ApiTestHelper for API integration tests
type ApiTestHelper struct {
	engine       *gin.Engine
	dbService    coreServices.DatabaseService
	ctx          context.Context
	baseURL      string
	token        string
	refreshToken string
	userId       string
	tenantSlug   string
	t            *testing.T
}

// SetupApiTest returns a helper bound to the shared router and connection pool.
// Only the per-test state (tokens, tenant slug, *testing.T) is fresh.
func SetupApiTest(t *testing.T) *ApiTestHelper {
	// Get global config (environment already loaded by init())
	globalConfig := config.GetConfig()
	if globalConfig == nil {
		t.Fatal("Failed to load global configuration")
	}

	engine, dbService := sharedTestServer()

	helper := &ApiTestHelper{
		engine:    engine,
		dbService: dbService,
		ctx:       context.Background(),
		baseURL:   "/api/v1",
		t:         t,
	}

	// Clean database before test (silent) - skip if DEBUG_KEEP_DB_DATA is set
	skipClean := os.Getenv("DEBUG_KEEP_DB_DATA") == "true"
	if !skipClean {
		//_ = helper.CleanDatabase()
	} else {
		t.Logf("⚠️  DEBUG_KEEP_DB_DATA=true - Database will NOT be cleaned before test")
	}

	return helper
}

// Valkey returns the shared test Valkey (REDIS_URL in .env.test), nil when it is unset.
func (h *ApiTestHelper) Valkey() coreServices.ValkeyService {
	return sharedValkey
}

// Close is a no-op since database lifecycle is managed by testutil
func (h *ApiTestHelper) Close() {
	// Database cleanup is handled by cmd/testutil between test runs
}

// CleanDatabase removes all test data by truncating business logic tables
// Uses CleanDatabaseWithExclusions to exclude auth schema by default
func (h *ApiTestHelper) CleanDatabase() error {
	return h.CleanDatabaseWithExclusions("auth") // Default: exclude auth schema
}

// CleanDatabaseWithExclusions removes test data from all schemas except specified exclusions
// exclusions: schemas to exclude from truncation (e.g., "auth", "pg_catalog", "information_schema")
// Pass empty args to clean all schemas (not recommended unless you know what you're doing)
func (h *ApiTestHelper) CleanDatabaseWithExclusions(exclusions ...string) error {
	if h.dbService == nil {
		return nil
	}

	ctx := context.Background()

	// Build schema filter to exclude specified schemas
	excludeList := "'" + strings.Join(append(exclusions, "pg_catalog", "information_schema"), "','") + "'"
	schemaFilter := `schemaname NOT IN (` + excludeList + `)`

	// Get all tables from non-excluded schemas
	query := `
		SELECT schemaname, tablename FROM pg_tables 
		WHERE ` + schemaFilter + `
		  AND tablename NOT IN ('schema_migrations_core', 'schema_migrations_auth', 
		                         'schema_migrations_users', 'schema_migrations_tenancy', 
		                         'schema_migrations_tracking', 'schema_migrations_inventory')
		ORDER BY schemaname, tablename
	`

	rows, err := h.dbService.Query(ctx, query)
	if err != nil {
		// Silently fail if we can't query tables (may not exist yet in test env)
		return nil
	}
	defer rows.Close()

	// Truncate each table that exists
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			continue
		}

		// Truncate with CASCADE to handle foreign keys
		fullName := schema + "." + tableName
		_, _ = h.dbService.Execute(ctx, "TRUNCATE TABLE "+fullName+" CASCADE")
	}

	return nil
}

// CleanDatabaseForSchemas removes test data from specific schemas only
// schemas: schemas to clean (e.g., "inventory", "tracking")
// Always excludes: pg_catalog, information_schema
func (h *ApiTestHelper) CleanDatabaseForSchemas(schemas ...string) error {
	if h.dbService == nil || len(schemas) == 0 {
		return nil
	}

	ctx := context.Background()

	// Build schema filter - only specified schemas
	schemaList := "'" + strings.Join(schemas, "','") + "'"
	schemaFilter := `schemaname IN (` + schemaList + `)`

	// Get all tables from specified schemas
	query := `
		SELECT schemaname, tablename FROM pg_tables 
		WHERE ` + schemaFilter + `
		  AND tablename NOT IN ('schema_migrations_core', 'schema_migrations_auth', 
		                         'schema_migrations_users', 'schema_migrations_tenancy', 
		                         'schema_migrations_tracking', 'schema_migrations_inventory')
		ORDER BY schemaname, tablename
	`

	rows, err := h.dbService.Query(ctx, query)
	if err != nil {
		// Silently fail if we can't query tables (may not exist yet in test env)
		return nil
	}
	defer rows.Close()

	// Truncate each table that exists
	for rows.Next() {
		var schema, tableName string
		if err := rows.Scan(&schema, &tableName); err != nil {
			continue
		}

		// Truncate with CASCADE to handle foreign keys
		fullName := schema + "." + tableName
		_, _ = h.dbService.Execute(ctx, "TRUNCATE TABLE "+fullName+" CASCADE")
	}

	return nil
}

// DoRequest performs HTTP request and returns response
func (h *ApiTestHelper) DoRequest(method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var bodyReader io.Reader

	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, h.baseURL+path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en")

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Add authorization if token exists
	if h.token != "" && headers["Authorization"] == "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.token))
	}

	// Add tenant slug if set
	if h.tenantSlug != "" && headers["X-Tenant-Slug"] == "" {
		req.Header.Set("X-Tenant-Slug", h.tenantSlug)
	}

	w := httptest.NewRecorder()
	h.engine.ServeHTTP(w, req)

	return w
}

// DoRootRequest performs an unauthenticated request against the engine root,
// for the few routes that live outside /api/v1 — the /livez and /readyz probes,
// which carry no auth, no tenant and no version prefix on purpose.
func (h *ApiTestHelper) DoRootRequest(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)

	w := httptest.NewRecorder()
	h.engine.ServeHTTP(w, req)

	return w
}

// MultipartFile is one file part of a multipart upload: the form field it is
// posted under, the filename the server sees, and the bytes.
type MultipartFile struct {
	Field    string
	Filename string
	Content  []byte
}

// DoMultipartRequest performs a multipart/form-data request, for the endpoints
// that take an upload instead of a JSON body (e.g. POST /import). Auth and tenant
// headers are applied exactly as DoRequest applies them; the boundary-carrying
// Content-Type is set from the writer, so it must not be passed in headers.
func (h *ApiTestHelper) DoMultipartRequest(
	method, path string,
	fields map[string]string,
	files []MultipartFile,
	headers map[string]string,
) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for name, value := range fields {
		_ = writer.WriteField(name, value)
	}
	for _, file := range files {
		part, err := writer.CreateFormFile(file.Field, file.Filename)
		if err != nil {
			h.t.Fatalf("failed to build the %q part: %v", file.Field, err)
		}
		if _, err := part.Write(file.Content); err != nil {
			h.t.Fatalf("failed to write the %q part: %v", file.Field, err)
		}
	}
	if err := writer.Close(); err != nil {
		h.t.Fatalf("failed to close the multipart writer: %v", err)
	}

	req := httptest.NewRequest(method, h.baseURL+path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept-Language", "en")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if h.token != "" && headers["Authorization"] == "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.token))
	}
	if h.tenantSlug != "" && headers["X-Tenant-Slug"] == "" {
		req.Header.Set("X-Tenant-Slug", h.tenantSlug)
	}

	w := httptest.NewRecorder()
	h.engine.ServeHTTP(w, req)

	return w
}

// Register creates a test user and returns user data
func (h *ApiTestHelper) Register(email, password, firstName, lastName string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"email":      email,
		"password":   password,
		"first_name": firstName,
		"last_name":  lastName,
		"phone":      "+1234567890",
		"birthday":   "1990-01-15",
	}

	w := h.DoRequest("POST", "/auth/register", body, map[string]string{})

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		return nil, fmt.Errorf("register failed with status %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	return response, nil
}

// Login authenticates a user and stores tokens
func (h *ApiTestHelper) Login(email, password string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"email":     email,
		"password":  password,
		"device_id": uuid.New().String(),
	}

	w := h.DoRequest("POST", "/auth/login", body, map[string]string{})

	if w.Code != http.StatusOK {
		return nil, fmt.Errorf("login failed with status %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Store tokens
	if accessToken, ok := response["access_token"].(string); ok {
		h.token = accessToken
	}
	if refreshToken, ok := response["refresh_token"].(string); ok {
		h.refreshToken = refreshToken
	}
	if user, ok := response["user"].(map[string]interface{}); ok {
		if id, ok := user["id"].(string); ok {
			h.userId = id
		}
	}

	return response, nil
}

// LoginAsSuperAdmin logs in as the pre-seeded super_admin user
// Email: superadmin@test.local
// Password: SuperAdmin123!
func (h *ApiTestHelper) LoginAsSuperAdmin() (map[string]interface{}, error) {
	return h.Login("superadmin@test.local", "SuperAdmin123!")
}

// LoginAsAdmin logs in as the pre-seeded admin user
// Email: admin@test.local
// Password: Admin123!
func (h *ApiTestHelper) LoginAsAdmin() (map[string]interface{}, error) {
	return h.Login("admin@test.local", "Admin123!")
}

// RefreshToken refreshes the access token
func (h *ApiTestHelper) RefreshToken() (string, error) {
	body := map[string]interface{}{
		"refresh_token": h.refreshToken,
	}

	w := h.DoRequest("POST", "/auth/token/refresh", body, map[string]string{})

	if w.Code != http.StatusOK {
		return "", fmt.Errorf("refresh token failed with status %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if newToken, ok := response["access_token"].(string); ok {
		h.token = newToken
		return newToken, nil
	}

	return "", fmt.Errorf("access_token not found in response")
}

// Engine is the shared router, for a test that needs a real listener (a WebSocket upgrade).
func (h *ApiTestHelper) Engine() *gin.Engine {
	return h.engine
}

// Token is the signed-in user's access token.
func (h *ApiTestHelper) Token() string {
	return h.token
}

// ClearToken removes the stored token (useful after logout)
func (h *ApiTestHelper) ClearToken() {
	h.token = ""
	h.refreshToken = ""
}

// GetUserID returns the authenticated user's ID.
// If not cached from login, fetches it from GET /auth/profile.
func (h *ApiTestHelper) GetUserID() string {
	if h.userId != "" {
		return h.userId
	}
	w := h.DoRequest("GET", "/auth/profile", nil, map[string]string{})
	if w.Code != http.StatusOK {
		return ""
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if id, ok := resp["id"].(string); ok {
		h.userId = id
	}
	return h.userId
}

// SetTenantSlug stores the tenant slug so DoRequest automatically includes X-Tenant-Slug header.
// DB exposes the shared pool for the few assertions no endpoint can make — a column the API never
// returns, such as a session's client_type. Everything a handler owns is still asserted through it.
func (h *ApiTestHelper) DB() coreServices.DatabaseService {
	return h.dbService
}

func (h *ApiTestHelper) SetTenantSlug(slug string) *ApiTestHelper {
	h.tenantSlug = slug
	return h
}

// GetUserFromDB retrieves user directly from database (via API call)
// This is a placeholder for real database queries
func (h *ApiTestHelper) GetUserFromDB(email string) (map[string]interface{}, error) {
	// For tests, we would retrieve user via API endpoint
	// This is commented out as tests should use API endpoints, not direct DB queries
	return map[string]interface{}{
		"email": email,
	}, nil
}

// AssertStatusCode asserts HTTP status code
func AssertStatusCode(t *testing.T, expected int, actual int, body string) {
	if expected != actual {
		t.Errorf("Expected status code %d, got %d. Response: %s", expected, actual, body)
	}
}

// AssertResponseHasField asserts response has a specific field
func AssertResponseHasField(t *testing.T, response map[string]interface{}, fieldName string) interface{} {
	value, exists := response[fieldName]
	if !exists {
		t.Errorf("Response missing field: %s", fieldName)
	}
	return value
}

// AssertErrorMessage asserts error response contains expected message
func AssertErrorMessage(t *testing.T, response map[string]interface{}, expectedMessage string) {
	if errData, ok := response["error"].(map[string]interface{}); ok {
		if message, ok := errData["message"].(string); ok {
			if message != expectedMessage && !bytes.Contains([]byte(message), []byte(expectedMessage)) {
				t.Errorf("Expected error message '%s', got '%s'", expectedMessage, message)
			}
			return
		}
	}
	t.Error("Response does not contain expected error structure")
}

// SetSystemRole gives an account a platform role (super_admin): no endpoint grants one.
func (h *ApiTestHelper) SetSystemRole(email, role string) error {
	_, err := h.DB().Execute(h.ctx, `UPDATE auth.users SET system_role = $2 WHERE email = $1`, email, role)
	return err
}

// UserLocale is the language an account's emails are written in (APP-009 D2); no endpoint reads it.
func (h *ApiTestHelper) UserLocale(email string) (string, error) {
	var locale string
	err := h.DB().QueryRow(h.ctx, `SELECT locale FROM auth.users WHERE email = $1`, email).Scan(&locale)
	return locale, err
}
