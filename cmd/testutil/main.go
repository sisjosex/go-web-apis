// filepath: cmd/testutil/main.go
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	coreConfig "josex/web/modules/core/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage"
	coreUtils "josex/web/modules/core/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	setup := flag.Bool("setup", false, "Setup test database + migrate (first time only)")
	reset := flag.Bool("reset", false, "Reset test database (drop, recreate, migrate)")
	clean := flag.Bool("clean", false, "Clean test database (drop & recreate, no migrate)")
	migrate := flag.Bool("migrate", false, "Run migrations only (on existing DB)")
	flag.Parse()

	switch {
	case *setup:
		setupTestDB()
	case *reset:
		resetTestDB()
	case *clean:
		cleanTestDB()
	case *migrate:
		runMigrations()
	default:
		printUsage()
	}
}

func setupTestDB() {
	prepareTestDB("🔧 Setting up test database...", "✅ Test database setup complete!")
}

func resetTestDB() {
	prepareTestDB("🔄 Resetting test database...", "✅ Test database reset and ready!")
}

// prepareTestDB is the shared logic for setup and reset: recreate + migrate + seed
func prepareTestDB(startMsg, doneMsg string) {
	fmt.Println(startMsg)
	os.Setenv("ENV_FILE", ".env.test")
	coreUtils.LoadEnv()
	dbName := getDatabaseName()
	recreateDatabase(dbName)
	runMigrations()
	seedDatabase()
	emptyTestBuckets()
	fmt.Println(doneMsg)
}

// emptyTestBuckets leaves the test buckets as empty as the database just recreated (INFRA-007): every
// object in them belonged to a row that is gone. Storage down is a warning, not a failed reset — the
// suites that need it fail on their own, and the rest do not need it.
func emptyTestBuckets() {
	cfg := coreConfig.LoadCoreConfig()
	client, err := storage.NewClient(cfg.StorageEndpoint, cfg.StorageAccessKey, cfg.StorageSecretKey)
	if err != nil {
		log.Printf("⚠️  test buckets not emptied: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, bucket := range []string{cfg.StorageMediaBucket, cfg.StorageDocumentsBucket} {
		if err := client.EmptyTestBucket(ctx, bucket); err != nil {
			log.Printf("⚠️  test bucket not emptied: %v", err)
		}
	}
}

func cleanTestDB() {
	fmt.Println("🧹 Cleaning test database...")
	os.Setenv("ENV_FILE", ".env.test")
	coreUtils.LoadEnv()
	dbName := getDatabaseName()
	recreateDatabase(dbName)
	fmt.Println("✅ Test database cleaned and ready!")
}

// getDatabaseName extracts database name from DATABASE_URL environment variable
func getDatabaseName() string {
	databaseURL := coreUtils.GetEnv("DATABASE_URL", "")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL not set in .env.test")
	}

	dbName := extractDatabaseName(databaseURL)
	if dbName == "" {
		log.Fatal("❌ Unable to extract database name from DATABASE_URL")
	}

	return dbName
}

// recreateDatabase drops and recreates the test database
func recreateDatabase(dbName string) {
	// First, terminate all connections to the database
	fmt.Printf("Terminating connections to database: %s...\n", dbName)
	terminateConnections(dbName)

	fmt.Printf("Dropping test database: %s...\n", dbName)
	execSQL("postgres", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", dbName))

	fmt.Printf("Creating test database: %s...\n", dbName)
	execSQL("postgres", fmt.Sprintf("CREATE DATABASE %s;", dbName))
}

// terminateConnections terminates all connections to the database
func terminateConnections(dbName string) {
	sql := fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		AND pid <> pg_backend_pid();
	`, dbName)
	execSQL("postgres", sql)
}

func runMigrations() {
	fmt.Println("📦 Running migrations...")

	// Load DATABASE_URL from environment
	databaseURL := coreUtils.GetEnv("DATABASE_URL", "")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL not set")
	}

	// Open database connection
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}

	// Load core config with test environment
	config := coreConfig.LoadCoreConfig()

	// Create migration service
	migrationService := coreServices.NewMigrationService(db, config)

	// Run migrations
	if err := migrationService.RunMigrations(); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	fmt.Println("✅ Migrations complete!")
}

func seedDatabase() {
	fmt.Println("🌱 Seeding test data...")
	databaseURL := coreUtils.GetEnv("DATABASE_URL", "")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}

	if err := seedTestData(db); err != nil {
		log.Fatalf("❌ Seeding failed: %v", err)
	}
}

// execSQL runs one statement through psql against the server DATABASE_URL names. Host and port are
// passed explicitly: without them psql falls back to libpq's defaults, so a DATABASE_URL pointing
// anywhere but localhost:5432 would drop and create the database on a different server than the one
// the migrations then connect to.
func execSQL(database, query string) {
	databaseURL := coreUtils.GetEnv("DATABASE_URL", "")
	user, password := extractCredentials(databaseURL)
	host, port := extractHostPort(databaseURL)

	env := os.Environ()
	env = append(env, "PGPASSWORD="+password)

	cmd := exec.Command("psql", "-h", host, "-p", port, "-U", user, "-d", database, "-c", query)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("⚠️  SQL execution note: %v", err)
	}
}

// extractHostPort parses the server a PostgreSQL connection URL points at, falling back to the
// local defaults when the URL says nothing.
func extractHostPort(databaseURL string) (host, port string) {
	host, port = "127.0.0.1", "5432"
	u, err := url.Parse(databaseURL)
	if err != nil {
		return host, port
	}
	if h := u.Hostname(); h != "" {
		host = h
	}
	if p := u.Port(); p != "" {
		port = p
	}
	return host, port
}

// extractCredentials parses user and password from a PostgreSQL connection URL
func extractCredentials(databaseURL string) (user, password string) {
	u, err := url.Parse(databaseURL)
	if err != nil || u.User == nil {
		return "postgres", "postgres" // safe fallback for local dev
	}
	user = u.User.Username()
	password, _ = u.User.Password()
	if user == "" {
		user = "postgres"
	}
	return user, password
}

// extractDatabaseName extracts database name from PostgreSQL connection URL
// Example: postgres://user:pass@localhost:5432/josex_test => josex_test
func extractDatabaseName(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil {
		log.Printf("⚠️  Failed to parse DATABASE_URL: %v", err)
		return ""
	}

	// Get path (e.g., /josex_test)
	path := strings.TrimPrefix(u.Path, "/")

	// Handle query parameters (e.g., sslmode=disable)
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	return path
}

func printUsage() {
	fmt.Println(`
Go Web API - Test Utility

Usage:
  go run cmd/testutil/main.go -setup       Setup test database + migrate (first time)
  go run cmd/testutil/main.go -reset       Reset test database (drop, create, migrate)
  go run cmd/testutil/main.go -clean       Clean test database (drop & recreate only)
  go run cmd/testutil/main.go -migrate     Run migrations only (existing DB)

Examples:
  # First time setup
  go run cmd/testutil/main.go -setup

  # Before running tests (cleans all data)
  go run cmd/testutil/main.go -reset

  # Quick clean without re-migrating
  go run cmd/testutil/main.go -clean

  # Just run migrations on existing DB
  go run cmd/testutil/main.go -migrate`)
}
