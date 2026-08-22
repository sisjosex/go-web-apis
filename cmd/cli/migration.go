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

// fallbackModules is used when ENABLED_MODULES is not set in the environment
const fallbackModules = "core,auth,tenancy,users,tracking,inventory,sales,purchasing"

// CmdMigration handles the `migration` (alias: m) subcommand
func CmdMigration(args []string) {
	fs := flag.NewFlagSet("migration", flag.ExitOnError)
	module := fs.String("module", "", "Target module name")
	name := fs.String("name", "", "Migration description, e.g. add_refresh_tokens")
	listFlag := fs.Bool("list", false, "List available modules and exit")

	modules := getEnabledModules()

	fs.Usage = func() {
		fmt.Println("📦 Migration Generator")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli m -module=<module> -name=<description>")
		fmt.Println("\nFlags:")
		fmt.Println("  -module string    Target module [required]")
		fmt.Println("  -name   string    Migration description, e.g. add_refresh_tokens [required]")
		fmt.Println("  -list             List available modules and exit")
		fmt.Println("\nAvailable modules:")
		for _, m := range modules {
			fmt.Printf("  • %s\n", m)
		}
		fmt.Println("\nExamples:")
		fmt.Println("  go run ./cmd/cli m -module=auth  -name=add_refresh_tokens")
		fmt.Println("  go run ./cmd/cli m -module=sales -name=add_discount_table")
		fmt.Println("  go run ./cmd/cli m -list")
	}

	fs.Parse(args)

	if *listFlag {
		fmt.Println("📦 Available modules (ENABLED_MODULES):")
		for _, m := range modules {
			fmt.Printf("  • %s\n", m)
		}
		return
	}

	if *module == "" || *name == "" {
		fmt.Println("❌ Both -module and -name are required")
		fmt.Println()
		fs.Usage()
		os.Exit(1)
	}

	if !isModuleEnabled(modules, *module) {
		fmt.Printf("❌ Unknown module '%s'. Run with -list to see available modules.\n", *module)
		os.Exit(1)
	}

	gen := &migrationGenerator{module: *module, name: sanitizeName(*name)}
	if err := gen.generate(); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

// getEnabledModules reads the module list from ENABLED_MODULES env var.
// Falls back to the full default list when the var is not set.
func getEnabledModules() []string {
	env := os.Getenv("ENABLED_MODULES")
	if env == "" {
		env = fallbackModules
	}
	var modules []string
	for _, m := range strings.Split(env, ",") {
		if m = strings.TrimSpace(m); m != "" {
			modules = append(modules, m)
		}
	}
	return modules
}

func isModuleEnabled(modules []string, module string) bool {
	for _, m := range modules {
		if m == module {
			return true
		}
	}
	return false
}

// migrationGenerator creates the up/down SQL files for a migration
type migrationGenerator struct {
	module string
	name   string
}

func (mg *migrationGenerator) generate() error {
	timestamp := time.Now().Format("20060102150405")
	dir := filepath.Join("modules", mg.module, "migrations")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	upPath := filepath.Join(dir, fmt.Sprintf("%s_%s.up.sql", timestamp, mg.name))
	downPath := filepath.Join(dir, fmt.Sprintf("%s_%s.down.sql", timestamp, mg.name))

	if err := os.WriteFile(upPath, []byte(mg.upTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to write up migration: %w", err)
	}
	if err := os.WriteFile(downPath, []byte(mg.downTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to write down migration: %w", err)
	}

	fmt.Printf("\n🎉 Migration created: %s\n", mg.name)
	fmt.Printf("   ↑ %s\n", upPath)
	fmt.Printf("   ↓ %s\n\n", downPath)
	return nil
}

func (mg *migrationGenerator) upTemplate() string {
	return fmt.Sprintf(`-- Migration: %s
-- Module:    %s
-- Created:   %s

-- TODO: Write your migration SQL here
-- Example table:
-- CREATE TABLE IF NOT EXISTS %s.my_table (
--     id         UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name       VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION %s.sp_my_proc() RETURNS TABLE(...) LANGUAGE plpgsql AS $$
-- BEGIN
--     ...
-- END;
-- $$;
`,
		mg.name, mg.module, time.Now().Format("2006-01-02 15:04:05"),
		mg.module, mg.module)
}

func (mg *migrationGenerator) downTemplate() string {
	return fmt.Sprintf(`-- Rollback: %s
-- Module:    %s

-- TODO: Write your rollback SQL here
-- Example:
-- DROP TABLE IF EXISTS %s.my_table;
-- DROP FUNCTION IF EXISTS %s.sp_my_proc;
`,
		mg.name, mg.module, mg.module, mg.module)
}

// sanitizeName replaces any non-alphanumeric characters with underscores
func sanitizeName(name string) string {
	name = regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(name, "_")
	name = regexp.MustCompile(`_+`).ReplaceAllString(name, "_")
	return strings.Trim(name, "_")
}
