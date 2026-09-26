package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	coreConfig "josex/web/modules/core/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Module represents a deployable module with its own migrations
type Module struct {
	Name    string
	Path    string
	Enabled bool
}

// MigrationService handles modular migrations
type MigrationService struct {
	modules []Module
	db      *sql.DB
}

// discoverAvailableModules scans the modules/ directory to find all available modules
func discoverAvailableModules() map[string]string {
	availableModules := make(map[string]string)
	modulesDir := "modules"

	// Read modules directory
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		log.Printf("⚠️  Warning: Could not read modules directory: %v", err)
		return availableModules
	}

	// Scan each directory in modules/
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		// Use forward slashes for cross-platform compatibility with golang-migrate
		migrationPath := filepath.ToSlash(filepath.Join(modulesDir, moduleName, "migrations"))

		// Check if migrations directory exists
		if _, err := os.Stat(filepath.FromSlash(migrationPath)); err == nil {
			availableModules[moduleName] = migrationPath
		}
	}

	return availableModules
}

// NewMigrationService creates a migration service with auto-discovered modules from filesystem
func NewMigrationService(db *sql.DB, config *coreConfig.CoreConfig) *MigrationService {
	// Auto-discover all available modules by scanning modules/ directory
	allModules := discoverAvailableModules()

	// Build enabled modules list from config
	modules := []Module{}

	// Core is always enabled first
	if config.IsModuleEnabled("core") {
		if path, exists := allModules["core"]; exists {
			modules = append(modules, Module{
				Name:    "core",
				Path:    path,
				Enabled: true,
			})
		}
	}

	// Add other enabled modules in order specified in config
	for _, moduleName := range config.EnabledModules {
		if moduleName == "core" {
			continue // Already added
		}
		// Skip modules that are excluded from migration (e.g., tracking in Main DB)
		if config.IsModuleExcludedFromMigration(moduleName) {
			log.Printf("⏭️  Skipping migrations for module '%s' (excluded by EXCLUDED_FROM_MIGRATION)", moduleName)
			continue
		}
		if path, exists := allModules[moduleName]; exists {
			modules = append(modules, Module{
				Name:    moduleName,
				Path:    path,
				Enabled: true,
			})
		} else {
			log.Printf("⚠️  Warning: Module '%s' is enabled in config but not found in filesystem", moduleName)
		}
	}

	return &MigrationService{
		db:      db,
		modules: modules,
	}
}

// EnableModule enables a specific module for deployment
func (ms *MigrationService) EnableModule(moduleName string) {
	for i := range ms.modules {
		if ms.modules[i].Name == moduleName {
			ms.modules[i].Enabled = true
			return
		}
	}
}

// DisableModule disables a specific module (except core)
func (ms *MigrationService) DisableModule(moduleName string) error {
	if moduleName == "core" {
		return fmt.Errorf("cannot disable core module")
	}
	for i := range ms.modules {
		if ms.modules[i].Name == moduleName {
			ms.modules[i].Enabled = false
			return nil
		}
	}
	return fmt.Errorf("module %s not found", moduleName)
}

// RunMigrations executes migrations for all enabled modules in order
func (ms *MigrationService) RunMigrations() error {
	log.Println("🚀 Starting modular migrations...")

	// Execute migrations for each enabled module
	for _, module := range ms.modules {
		if !module.Enabled {
			log.Printf("⏭️  Skipping disabled module: %s", module.Name)
			continue
		}

		// Check if migration path exists
		if _, err := os.Stat(module.Path); os.IsNotExist(err) {
			log.Printf("⚠️  No migrations found for module: %s (path: %s)", module.Name, module.Path)
			continue
		}

		// Count migration files
		files, err := os.ReadDir(module.Path)
		if err != nil {
			return fmt.Errorf("failed to read migrations for module %s: %w", module.Name, err)
		}

		upFiles := 0
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".up.sql") {
				upFiles++
			}
		}

		if upFiles == 0 {
			log.Printf("⏭️  No migration files for module: %s", module.Name)
			continue
		}

		log.Printf("📦 Running migrations for module: %s (%d files)", module.Name, upFiles)

		if err := ms.runModuleMigrations(module); err != nil {
			return fmt.Errorf("migration failed for module %s: %w", module.Name, err)
		}

		log.Printf("✅ Module %s migrations applied successfully", module.Name)
	}

	return nil
}

// runModuleMigrations executes migrations for a specific module
func (ms *MigrationService) runModuleMigrations(module Module) error {
	// Create postgres driver with custom schema table per module
	driver, err := postgres.WithInstance(ms.db, &postgres.Config{
		MigrationsTable: fmt.Sprintf("schema_migrations_%s", module.Name),
	})
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}

	// Use the path as-is (already in forward slash format from discoverAvailableModules)
	sourceURL := fmt.Sprintf("file://%s", module.Path)
	log.Printf("🔍 Migration source URL for %s: %s", module.Name, sourceURL)

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	// Don't defer close - will close the shared DB connection

	// Run migrations
	err = m.Up()
	log.Printf("🔍 Migration result for %s: %v", module.Name, err)

	if err != nil && err != migrate.ErrNoChange {
		// A dirty version is a migration whose SQL failed half-way. It is never forced clean here:
		// forcing past it applies every later migration on top of objects that do not exist, and
		// the database ends up a mix of old tables and new procedures (TRACK-007 on a Postgres
		// without PostGIS did exactly that). The startup loop keeps retrying and logging until a
		// person runs the failed file's .down.sql, fixes the cause and resets the version:
		//   UPDATE schema_migrations_<module> SET version = <previous version>, dirty = false;
		if strings.Contains(err.Error(), "Dirty database") {
			return fmt.Errorf("module %s has a half-applied migration and needs a manual repair: %w", module.Name, err)
		}
		return fmt.Errorf("migration execution failed: %w", err)
	}

	return nil
}

// GetModuleStatus returns the status of all modules
func (ms *MigrationService) GetModuleStatus() []ModuleStatus {
	var statuses []ModuleStatus

	for _, module := range ms.modules {
		status := ModuleStatus{
			Name:    module.Name,
			Path:    module.Path,
			Enabled: module.Enabled,
		}

		// Check if path exists
		if _, err := os.Stat(module.Path); os.IsNotExist(err) {
			status.Status = "missing"
			statuses = append(statuses, status)
			continue
		}

		// Count migration files
		files, err := os.ReadDir(module.Path)
		if err != nil {
			status.Status = "error"
			statuses = append(statuses, status)
			continue
		}

		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".up.sql") {
				status.MigrationCount++
			}
		}

		if module.Enabled {
			status.Status = "active"
		} else {
			status.Status = "disabled"
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// ModuleStatus represents the current state of a module
type ModuleStatus struct {
	Name           string
	Path           string
	Enabled        bool
	Status         string // "active", "disabled", "missing", "error"
	MigrationCount int
}

// PrintModuleStatus displays module status in a readable format
func (ms *MigrationService) PrintModuleStatus() {
	statuses := ms.GetModuleStatus()

	log.Println("📋 Module Status:")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, status := range statuses {
		emoji := "✅"
		if !status.Enabled {
			emoji = "⏸️"
		}
		if status.Status == "missing" {
			emoji = "❌"
		}

		log.Printf("%s %-15s | Status: %-8s | Migrations: %d | Path: %s",
			emoji, status.Name, status.Status, status.MigrationCount, status.Path)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// RollbackModule rolls back the last migration of a specific module
func (ms *MigrationService) RollbackModule(moduleName string) error {
	// Find the module
	var targetModule *Module
	for i := range ms.modules {
		if ms.modules[i].Name == moduleName {
			targetModule = &ms.modules[i]
			break
		}
	}

	if targetModule == nil {
		return fmt.Errorf("module %s not found", moduleName)
	}

	log.Printf("⏪ Rolling back last migration for module: %s", moduleName)

	driver, err := postgres.WithInstance(ms.db, &postgres.Config{
		MigrationsTable: fmt.Sprintf("schema_migrations_%s", targetModule.Name),
	})
	if err != nil {
		return fmt.Errorf("failed to create driver: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", targetModule.Path)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	// Don't defer close

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	log.Printf("✅ Rollback completed for module: %s", moduleName)
	return nil
}

// GetEnabledModules returns a list of enabled module names
func (ms *MigrationService) GetEnabledModules() []string {
	var enabled []string
	for _, module := range ms.modules {
		if module.Enabled {
			enabled = append(enabled, module.Name)
		}
	}
	sort.Strings(enabled)
	return enabled
}
