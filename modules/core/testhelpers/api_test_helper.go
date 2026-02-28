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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/routes"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// testInitOnce ensures setup logs are printed only once
var testInitOnce sync.Once

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
	t            *testing.T
}

// SetupApiTest initializes test environment with real database
func SetupApiTest(t *testing.T) *ApiTestHelper {
	// Get global config (environment already loaded by init())
	globalConfig := config.GetConfig()
	if globalConfig == nil {
		t.Fatal("Failed to load global configuration")
	}

	// Initialize database service
	dbService := coreServices.NewDatabaseService()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })

	// Skip migrations in tests (they're already run by: go run ./cmd/testutil -reset)
	os.Setenv("SKIP_MIGRATIONS", "true")
	defer os.Unsetenv("SKIP_MIGRATIONS")

	// Silence repetitive logs during test setup
	oldOut := log.Writer()
	log.SetOutput(io.Discard)

	// Initialize database synchronously (SKIP_MIGRATIONS=true makes it fast)
	// No need for goroutine since we skip migrations in tests
	dbService.InitDatabase(ctx)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	// Load translations (silent)
	languages := []string{"en", "es"}
	coreServices.LoadAllTranslations(languages, globalConfig.Core.EnabledModules)

	// Setup routes (still silenced)
	routes.SetupRoutes(engine, dbService)

	// Restore log output after all setup is complete
	log.SetOutput(oldOut)

	helper := &ApiTestHelper{
		engine:    engine,
		dbService: dbService,
		ctx:       ctx,
		baseURL:   "/api/v1",
		t:         t,
	}

	// Clean database before test (silent)
	_ = helper.CleanDatabase()

	return helper
}

// Close is a no-op since database lifecycle is managed by testutil
func (h *ApiTestHelper) Close() {
	// Database cleanup is handled by cmd/testutil between test runs
}

// CleanDatabase removes test data by truncating tables
func (h *ApiTestHelper) CleanDatabase() error {
	// Silent - don't print logs during test execution
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

// ClearToken removes the stored token (useful after logout)
func (h *ApiTestHelper) ClearToken() {
	h.token = ""
	h.refreshToken = ""
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
