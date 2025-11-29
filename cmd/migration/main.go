package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MigrationGenerator generates migration files for modules
type MigrationGenerator struct {
	module string
	name   string
}

func main() {
	// Define flags
	module := flag.String("module", "", "Module name (core, auth, users)")
	name := flag.String("name", "", "Migration name (e.g., add_refresh_tokens)")

	// Parse flags
	flag.Parse()

	// Validate inputs
	if *module == "" || *name == "" {
		fmt.Println("❌ Error: Both -module and -name are required")
		fmt.Println("\nUsage:")
		fmt.Println("  go run . migration -module=<module> -name=<migration_name>")
		fmt.Println("\nExamples:")
		fmt.Println("  go run . migration -module=auth -name=add_refresh_tokens")
		fmt.Println("  go run . migration -module=users -name=add_avatar_field")
		fmt.Println("  go run . migration -module=core -name=add_postgis_extension")
		fmt.Println("\nAvailable modules: core, auth, users")
		os.Exit(1)
	}

	// Validate module
	validModules := map[string]bool{"core": true, "auth": true, "users": true}
	if !validModules[*module] {
		fmt.Printf("❌ Error: Invalid module '%s'. Valid modules: core, auth, users\n", *module)
		os.Exit(1)
	}

	gen := &MigrationGenerator{
		module: *module,
		name:   sanitizeName(*name),
	}

	if err := gen.Generate(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func sanitizeName(name string) string {
	// Replace any non-alphanumeric characters with underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := re.ReplaceAllString(name, "_")
	// Remove consecutive underscores
	re = regexp.MustCompile(`_+`)
	sanitized = re.ReplaceAllString(sanitized, "_")
	// Trim underscores from start and end
	return strings.Trim(sanitized, "_")
}

func (mg *MigrationGenerator) Generate() error {
	// Generate timestamp
	timestamp := time.Now().Format("20060102150405")

	// Build file names
	upFile := fmt.Sprintf("%s_%s.up.sql", timestamp, mg.name)
	downFile := fmt.Sprintf("%s_%s.down.sql", timestamp, mg.name)

	// Module path
	modulePath := filepath.Join("modules", mg.module, "migrations")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(modulePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Full file paths
	upFilePath := filepath.Join(modulePath, upFile)
	downFilePath := filepath.Join(modulePath, downFile)

	// Templates
	upTemplate := mg.getUpTemplate()
	downTemplate := mg.getDownTemplate()

	// Create files
	if err := os.WriteFile(upFilePath, []byte(upTemplate), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	if err := os.WriteFile(downFilePath, []byte(downTemplate), 0644); err != nil {
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	// Print success message
	mg.printSuccess(upFilePath, downFilePath, timestamp)

	return nil
}

func (mg *MigrationGenerator) getUpTemplate() string {
	return fmt.Sprintf(`-- Migration: %s
-- Module: %s
-- Created: %s

-- TODO: Write your migration SQL here
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

`, mg.name, mg.module, time.Now().Format("2006-01-02 15:04:05"))
}

func (mg *MigrationGenerator) getDownTemplate() string {
	return fmt.Sprintf(`-- Rollback: %s
-- Module: %s

-- TODO: Write your rollback SQL here
-- Example:
-- DROP TABLE IF EXISTS auth.my_table;

`, mg.name, mg.module)
}

func (mg *MigrationGenerator) printSuccess(upPath, downPath, timestamp string) {
	fmt.Println()
	fmt.Println("🎉 Migration created successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📦 Module:    %s\n", mg.module)
	fmt.Printf("📝 Name:      %s\n", mg.name)
	fmt.Printf("🕐 Timestamp: %s\n", timestamp)
	fmt.Println()
	fmt.Println("📁 Files created:")
	fmt.Printf("   ↑ %s\n", upPath)
	fmt.Printf("   ↓ %s\n", downPath)
	fmt.Println()
	fmt.Println("📝 Next steps:")
	fmt.Printf("   1. Edit %s to add your SQL\n", filepath.Base(upPath))
	fmt.Printf("   2. Edit %s for rollback\n", filepath.Base(downPath))
	fmt.Println("   3. Run: go run .")
	fmt.Println()
}
