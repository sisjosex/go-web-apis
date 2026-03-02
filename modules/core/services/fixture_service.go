package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FixtureService handles test data fixtures for all modules
type FixtureService struct {
	db *sql.DB
}

// NewFixtureService creates a fixture service
func NewFixtureService(db *sql.DB) *FixtureService {
	return &FixtureService{db: db}
}

// discoverFixtures scans all modules for test fixtures
// Returns a map of module -> []fixture files
func (fs *FixtureService) discoverFixtures() map[string][]string {
	fixturesByModule := make(map[string][]string)
	modulesDir := "modules"

	// Try to find modules directory from different working directories
	if _, err := os.Stat(modulesDir); err != nil {
		// Try one level up
		modulesDir = filepath.Join("..", modulesDir)
		if _, err := os.Stat(modulesDir); err != nil {
			log.Printf("⚠️  Warning: Could not locate modules directory")
			return fixturesByModule
		}
	}

	// Read modules directory
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		log.Printf("⚠️  Warning: Could not read modules directory: %v", err)
		return fixturesByModule
	}

	// Scan each module for fixtures directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		fixturesDir := filepath.Join(modulesDir, moduleName, "tests", "fixtures")

		// Check if fixtures directory exists
		if _, err := os.Stat(fixturesDir); err != nil {
			continue // No fixtures directory - skip this module
		}

		// Read all .sql files in the fixtures directory
		fixtureFiles, err := os.ReadDir(fixturesDir)
		if err != nil {
			log.Printf("⚠️  Warning: Could not read fixtures directory for module '%s': %v", moduleName, err)
			continue
		}

		var sqlFiles []string
		for _, file := range fixtureFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
				sqlFiles = append(sqlFiles, filepath.Join(fixturesDir, file.Name()))
			}
		}

		// Sort files to ensure consistent order
		sort.Strings(sqlFiles)

		if len(sqlFiles) > 0 {
			fixturesByModule[moduleName] = sqlFiles
		}
	}

	return fixturesByModule
}

// LoadFixtures loads and executes all discovered fixture files
func (fs *FixtureService) LoadFixtures() error {
	fixtures := fs.discoverFixtures()

	if len(fixtures) == 0 {
		log.Println("ℹ️  No test fixtures found")
		return nil
	}

	log.Println("🌱 Loading test fixtures...")

	// Process modules in a consistent order (alphabetically)
	var modules []string
	for moduleName := range fixtures {
		modules = append(modules, moduleName)
	}
	sort.Strings(modules)

	for _, moduleName := range modules {
		files := fixtures[moduleName]

		for _, filePath := range files {
			fileName := filepath.Base(filePath)
			log.Printf("  📄 %s/%s", moduleName, fileName)

			// Read fixture file
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read fixture file %s: %w", filePath, err)
			}

			// Execute fixture SQL
			if _, err := fs.db.Exec(string(content)); err != nil {
				log.Printf("⚠️  Warning: Error loading fixture %s (may be expected if data exists): %v", filePath, err)
				// Don't return error - fixtures should be idempotent
			} else {
				log.Printf("  ✅ Fixture %s loaded successfully", fileName)
			}
		}
	}

	log.Println("🌱 Test fixtures loading complete!")
	return nil
}
