// filepath: cmd/testutil/main.go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"

	coreConfig "josex/web/modules/core/config"
	coreServices "josex/web/modules/core/services"
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
	fmt.Println("🔧 Setting up test database...")
	os.Setenv("ENV_FILE", ".env.test")
	coreUtils.LoadEnv()
	dbName := getDatabaseName()
	recreateDatabase(dbName)
	runMigrations()
	seedDatabase()
	fmt.Println("✅ Test database setup complete!")
}

func resetTestDB() {
	fmt.Println("🔄 Resetting test database...")
	os.Setenv("ENV_FILE", ".env.test")
	coreUtils.LoadEnv()
	dbName := getDatabaseName()
	recreateDatabase(dbName)
	runMigrations()
	seedDatabase()
	fmt.Println("✅ Test database reset and ready!")
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

func execSQL(database, query string) {
	// Prepare environment with password from DATABASE_URL
	env := os.Environ()
	env = append(env, "PGPASSWORD=postgres")

	cmd := exec.Command(
		"psql",
		"-U", "postgres",
		"-d", database,
		"-c", query,
	)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("⚠️  SQL execution note: %v", err)
	}
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
