// Package main: CLI development tool for migrations and tenant management
//
// Subcommands:
//   - migration: Generate new database migration files (independent, no config needed)
//   - migrate:   Run the modular migrations on the ENV_FILE database once and exit
//   - tenant:    Manage tenant instances and run tenant-specific migrations (requires .env.platform)
//   - jobs:      Run one scheduled job and exit (requires .env.platform)
//   - geo:       Import the address-search places of a geo build (.env.tenant)
//   - tracking:  Tracking maintenance: queue the planned route lines (requires .env.platform)
//   - seed-dev:  Write the emulator's dev accounts into the .env.platform database
//
// Each subcommand has its own handler file:
//   - migration.go:    Migration generation logic
//   - migrate.go:      One-shot schema migration (the deploy's migrate job)
//   - tenant_mgmt.go:  Tenant management and operations
//   - jobs.go:         Scheduled jobs cron runs one at a time
//   - geo.go:          Geo places import
//   - tracking.go:     Tracking maintenance
//   - seed_dev.go:     Dev accounts (seed_dev.sql)
//   - main.go:         CLI routing and help (this file)
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printMainUsage()
		os.Exit(0)
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "migration", "m":
		CmdMigration(args)
	case "migrate", "mig":
		CmdMigrate(args)
	case "tenant", "t":
		CmdTenant(args)
	case "jobs", "j":
		CmdJobs(args)
	case "geo":
		CmdGeo(args)
	case "tracking":
		CmdTracking(args)
	case "seed-dev":
		CmdSeedDev(args)
	case "help", "h", "-h", "--help":
		printMainUsage()
	default:
		fmt.Printf("❌ Unknown subcommand: %s\n\n", subcommand)
		printMainUsage()
		os.Exit(1)
	}
}

// printMainUsage displays the main CLI help
func printMainUsage() {
	fmt.Println("🛠️  Go Web API - Development CLI Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  go run ./cmd/cli <subcommand> [flags]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  migration, m    Create new database schema migrations")
	fmt.Println("  migrate, mig    Run the modular migrations on the ENV_FILE database")
	fmt.Println("  tenant, t       Manage tenant instances and run tenant migrations")
	fmt.Println("  jobs, j         Run a scheduled job once (cron calls these)")
	fmt.Println("  geo             Import a geo build's places into geo.places")
	fmt.Println("  tracking        Tracking maintenance (paths backfill)")
	fmt.Println("  seed-dev        Write the emulator's dev accounts (idempotent)")
	fmt.Println("  help, -h        Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  go run ./cmd/cli migration -module=auth -name=add_refresh_tokens")
	fmt.Println("  go run ./cmd/cli migrate")
	fmt.Println("  go run ./cmd/cli tenant -list")
	fmt.Println("  go run ./cmd/cli tenant -migrate all")
	fmt.Println("  go run ./cmd/cli jobs document-alerts")
	fmt.Println("  go run ./cmd/cli geo import docker/geo/data/current/places.geojsonl")
	fmt.Println("  go run ./cmd/cli tracking paths backfill")
	fmt.Println("\nRun 'go run ./cmd/cli <subcommand> -h' for subcommand help")
}
