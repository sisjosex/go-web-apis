// Package main: Migration generator
//
// This subcommand generates new SQL migration files for modules.
// It's independent and doesn't require loading application config.
//
// Files organized as:
//   - CmdMigration:     CLI command handler
//   - migrationGenerator: Core migration generation logic
//   - Helper functions: Template generation, success messages, validation
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

// CmdMigration handles the migration subcommand
func CmdMigration(args []string) {
	fs := flag.NewFlagSet("migration", flag.ExitOnError)
	module := fs.String("module", "", "Module name (core, auth, users, tracking)")
	name := fs.String("name", "", "Migration name (e.g., add_refresh_tokens)")

	fs.Usage = func() {
		fmt.Println("📦 Migration Generator - Create new database schema migrations")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli migration -module=<module> -name=<migration_name>")
		fmt.Println("\nFlags:")
		fmt.Println("  -module string    Module name (core, auth, users, tracking)")
		fmt.Println("  -name string      Migration name (e.g., add_refresh_tokens)")
		fmt.Println("\nExamples:")
		fmt.Println("  go run ./cmd/cli migration -module=auth -name=add_refresh_tokens")
		fmt.Println("  go run ./cmd/cli migration -module=users -name=add_avatar_field")
		fmt.Println("  go run ./cmd/cli migration -module=core -name=add_postgis_extension")
		fmt.Println("  go run ./cmd/cli migration -module=tracking -name=add_location_field")
	}

	fs.Parse(args)

	if *module == "" || *name == "" {
		fmt.Println("❌ Error: Both -module and -name flags are required")
		fs.Usage()
		os.Exit(1)
	}

	// Validate module
	validModules := map[string]bool{"core": true, "auth": true, "users": true, "tracking": true}
	if !validModules[*module] {
		fmt.Printf("❌ Error: Invalid module '%s'. Valid modules: core, auth, users, tracking\n", *module)
		os.Exit(1)
	}

	gen := newMigrationGenerator(*module, sanitizeName(*name))
	if err := gen.generate(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

// migrationGenerator handles migration file creation
type migrationGenerator struct {
	module string
	name   string
}

func newMigrationGenerator(module, name string) *migrationGenerator {
	return &migrationGenerator{
		module: module,
		name:   name,
	}
}

// generate creates the up and down migration files
func (mg *migrationGenerator) generate() error {
	timestamp := time.Now().Format("20060102150405")

	upFile := fmt.Sprintf("%s_%s.up.sql", timestamp, mg.name)
	downFile := fmt.Sprintf("%s_%s.down.sql", timestamp, mg.name)

	modulePath := filepath.Join("modules", mg.module, "migrations")

	if err := os.MkdirAll(modulePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	upFilePath := filepath.Join(modulePath, upFile)
	downFilePath := filepath.Join(modulePath, downFile)

	if err := os.WriteFile(upFilePath, []byte(mg.upTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to create up migration: %w", err)
	}

	if err := os.WriteFile(downFilePath, []byte(mg.downTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to create down migration: %w", err)
	}

	mg.printSuccess(upFilePath, downFilePath, timestamp)
	return nil
}

// upTemplate returns the SQL up migration template
func (mg *migrationGenerator) upTemplate() string {
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

// downTemplate returns the SQL down migration template
func (mg *migrationGenerator) downTemplate() string {
	return fmt.Sprintf(`-- Rollback: %s
-- Module: %s

-- TODO: Write your rollback SQL here
-- Example:
-- DROP TABLE IF EXISTS auth.my_table;

`, mg.name, mg.module)
}

// printSuccess displays success message with created file paths
func (mg *migrationGenerator) printSuccess(upPath, downPath, timestamp string) {
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
	fmt.Println("   3. Restart your application to run migrations")
	fmt.Println()
}

// sanitizeName cleans migration name of invalid characters
func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := re.ReplaceAllString(name, "_")
	re = regexp.MustCompile(`_+`)
	sanitized = re.ReplaceAllString(sanitized, "_")
	return strings.Trim(sanitized, "_")
}
