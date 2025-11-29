package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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

// NewMigrationService creates a migration service with configurable modules from CoreConfig
func NewMigrationService(db *sql.DB, config *coreConfig.CoreConfig) *MigrationService {
	// Define all available modules
	allModules := map[string]string{
		"core":  "modules/core/migrations",
		"auth":  "modules/auth/migrations",
		"users": "modules/users/migrations",
		// Future modules can be added here:
		// "notifications": "modules/notifications/migrations",
	}

	// Build enabled modules list from config
	modules := []Module{}

	// Core is always enabled first
	if config.IsModuleEnabled("core") {
		modules = append(modules, Module{
			Name:    "core",
			Path:    allModules["core"],
			Enabled: true,
		})
	}

	// Add other enabled modules
	for _, moduleName := range config.EnabledModules {
		if moduleName == "core" {
			continue // Already added
		}
		if path, exists := allModules[moduleName]; exists {
			modules = append(modules, Module{
				Name:    moduleName,
				Path:    path,
				Enabled: true,
			})
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

	log.Println("✅ All modular migrations completed successfully")
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

	// Use relative path (works better on Windows)
	sourceURL := fmt.Sprintf("file://%s", module.Path)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	// Don't defer close - will close the shared DB connection

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
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
