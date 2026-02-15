// Migration CLI - Generate new database schema migrations
//
// This CLI dynamically reads enabled modules from ENABLED_MODULES env var.
// It does NOT have a hardcoded list of modules.
//
// Usage:
//
//	go run ./cmd/migration -module=<module> -name=<description>
//	go run ./cmd/migration -list
//
// Examples:
//
//	go run ./cmd/migration -module=auth -name=add_refresh_tokens
//	go run ./cmd/migration -module=users -name=add_avatar_field
//	go run ./cmd/migration -module=tracking -name=add_location_index
//	go run ./cmd/migration -list
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"josex/web/modules/core/config"
	_ "josex/web/modules/core/utils"
)

func init() {
	// Set default ENABLED_MODULES if not set
	if os.Getenv("ENABLED_MODULES") == "" {
		os.Setenv("ENABLED_MODULES", "core,auth,users,tenancy,tracking")
	}
}

func main() {
	module := flag.String("module", "", "Module name (from ENABLED_MODULES)")
	name := flag.String("name", "", "Migration name (e.g., add_refresh_tokens)")
	help := flag.Bool("h", false, "Show help")
	listModules := flag.Bool("list", false, "List enabled modules and exit")

	flag.Parse()

	// Load core config to get enabled modules dynamically
	coreConfig := config.LoadCoreConfig()
	enabledModules := coreConfig.GetEnabledModules()

	if *help {
		printUsage(enabledModules)
		os.Exit(0)
	}

	if *listModules {
		printEnabledModules(enabledModules)
		os.Exit(0)
	}

	if *module == "" || *name == "" {
		fmt.Println("❌ Missing required flags: -module and -name")
		fmt.Println("Run with -h for help or -list to see enabled modules")
		os.Exit(1)
	}

	// Validate module is enabled (dynamically from config)
	if !isModuleEnabled(enabledModules, *module) {
		fmt.Printf("❌ Module '%s' is not enabled.\n\n", *module)
		printEnabledModules(enabledModules)
		os.Exit(1)
	}

	gen := newMigrationGenerator(*module, sanitizeName(*name))
	if err := gen.generate(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

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

func (mg *migrationGenerator) upTemplate() string {
	return fmt.Sprintf(`-- Migration: %s
-- Module: %s
-- Created: %s

-- TODO: Write your migration SQL here
-- Example table creation:
-- CREATE TABLE IF NOT EXISTS %s.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION %s.sp_operation() RETURNS TABLE(...) AS $$
-- BEGIN
--     -- Logic here
-- END;
-- $$ LANGUAGE plpgsql;

`, mg.name, mg.module, time.Now().Format("2006-01-02 15:04:05"), mg.module, mg.module)
}

func (mg *migrationGenerator) downTemplate() string {
	return fmt.Sprintf(`-- Rollback: %s
-- Module: %s

-- TODO: Write your rollback SQL here
-- Example table drop:
-- DROP TABLE IF EXISTS %s.my_table;

-- Example function drop:
-- DROP FUNCTION IF EXISTS %s.sp_operation CASCADE;

`, mg.name, mg.module, mg.module, mg.module)
}

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

func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	sanitized := re.ReplaceAllString(name, "_")
	re = regexp.MustCompile(`_+`)
	sanitized = re.ReplaceAllString(sanitized, "_")
	return strings.Trim(sanitized, "_")
}

func isModuleEnabled(enabledModules []string, module string) bool {
	for _, m := range enabledModules {
		if m == module {
			return true
		}
	}
	return false
}

func printEnabledModules(enabledModules []string) {
	fmt.Println("📦 Enabled modules (from ENABLED_MODULES env var):")
	fmt.Println()
	for _, module := range enabledModules {
		fmt.Printf("   • %s\n", module)
	}
	fmt.Println()
}

func printUsage(enabledModules []string) {
	fmt.Println("📦 Migration Generator - Create new database schema migrations")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/migration -module=<module> -name=<description>")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -module string    Module name (from ENABLED_MODULES) [required]")
	fmt.Println("  -name string      Migration name (e.g., add_refresh_tokens) [required]")
	fmt.Println("  -list             List enabled modules and exit")
	fmt.Println("  -h                Show this help")
	fmt.Println()
	fmt.Println("Enabled Modules (from ENABLED_MODULES env var):")
	for _, module := range enabledModules {
		fmt.Printf("  • %s\n", module)
	}
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./cmd/migration -module=auth -name=add_refresh_tokens")
	fmt.Println("  go run ./cmd/migration -module=users -name=add_avatar_field")
	fmt.Println("  go run ./cmd/migration -module=tracking -name=add_location_index")
	fmt.Println("  go run ./cmd/migration -list")
	fmt.Println()
}
