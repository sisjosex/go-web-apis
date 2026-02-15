// Package main: Tenant management
//
// This subcommand manages tenant instances and runs migrations on tenant databases.
// It requires .env.platform with TENANCY_ENABLED=true to operate.
//
// Files organized as:
//   - CmdTenant:                 CLI command handler
//   - runTenantOps:              Router to specific operations
//   - listTenants:               List all tenants with custom databases
//   - runMigrations:             Run migrations on specific tenant(s)
//   - Helper functions:          Database URL masking, display formatting
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"josex/web/modules/tenancy/interfaces"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyServices "josex/web/modules/tenancy/services"
)

// CmdTenant handles the tenant subcommand
func CmdTenant(args []string) {
	fs := flag.NewFlagSet("tenant", flag.ExitOnError)
	listCmd := fs.Bool("list", false, "List all tenants with custom databases")
	migrateCmd := fs.String("migrate", "", "Run migrations on tenant (slug) or 'all' for all tenants")
	tenant := fs.String("tenant", "", "Tenant slug for operations")

	fs.Usage = func() {
		fmt.Println("🏢 Tenant Management - Manage tenant instances and migrations")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli tenant [flags]")
		fmt.Println("\nFlags:")
		fmt.Println("  -list                    List all tenants with custom databases")
		fmt.Println("  -migrate <slug|all>      Run migrations on specific tenant or all tenants")
		fmt.Println("  -tenant <slug>           Specify tenant slug for operations")
		fmt.Println("\nExamples:")
		fmt.Println("  go run ./cmd/cli tenant -list")
		fmt.Println("  go run ./cmd/cli tenant -migrate acme")
		fmt.Println("  go run ./cmd/cli tenant -migrate all")
		fmt.Println("\nNote: Tenant creation and user management should be done via Platform API")
		fmt.Println("Note: This command requires .env.platform with TENANCY_ENABLED=true")
	}

	fs.Parse(args)

	// Default action: list all tenants
	if !*listCmd && *migrateCmd == "" {
		*listCmd = true
	}

	// Set environment for tenant operations (requires platform context)
	detectAndSetEnvForTenant()

	if err := runTenantOps(*listCmd, *migrateCmd, *tenant); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

// detectAndSetEnvForTenant ensures platform .env is loaded
func detectAndSetEnvForTenant() {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		return
	}
	os.Setenv("ENV_FILE", ".env.platform")
	os.Setenv("ENABLED_MODULES", "core,auth,users,tenancy")
}

// runTenantOps handles all tenant-related operations
func runTenantOps(list bool, migrate, tenantSlug string) error {
	// Load environment variables first
	utils.LoadEnv()

	// Check if tenancy is enabled
	tenancyEnabled := utils.GetEnv("TENANCY_ENABLED", "false")
	if tenancyEnabled != "true" {
		return fmt.Errorf("tenancy module is not enabled. Make sure you're using .env.platform with TENANCY_ENABLED=true")
	}

	// Initialize services - this will load full config which requires all .env vars
	ctx := context.Background()
	dbService := coreServices.NewDatabaseService()
	dbService.InitDatabase(ctx)
	defer dbService.CloseDatabase(ctx)

	// Initialize tenant repository and service
	tenantRepo := tenancyRepos.NewTenantRepository(dbService)
	tenantService := tenancyServices.NewTenantService(tenantRepo, dbService)

	// Route to appropriate operation
	switch {
	case list:
		return listTenants(ctx, tenantRepo)
	case migrate != "":
		return runMigrations(ctx, tenantService, tenantRepo, migrate)
	default:
		return nil
	}
}

// listTenants lists all tenants with custom databases
func listTenants(ctx context.Context, repo interfaces.TenantRepository) error {
	tenants, err := repo.ListTenantsWithCustomDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	if len(tenants) == 0 {
		fmt.Println("📋 No tenants with custom databases found")
		return nil
	}

	fmt.Println("📋 Tenants with custom databases:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, tenant := range tenants {
		status := "✅ Active"
		if !tenant.IsActive {
			status = "❌ Inactive"
		}
		fmt.Printf("  %s - %s (%s)\n", tenant.Slug, tenant.Name, status)
		if tenant.DatabaseURL != nil {
			fmt.Printf("    DB: %s\n", maskDatabaseURL(*tenant.DatabaseURL))
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}

// runMigrations runs migrations on specific tenant(s)
func runMigrations(ctx context.Context, service interfaces.TenantService, repo interfaces.TenantRepository, target string) error {
	if target == "all" {
		// Get all tenants with custom DB
		tenants, err := repo.ListTenantsWithCustomDB(ctx)
		if err != nil {
			return fmt.Errorf("failed to list tenants: %w", err)
		}

		if len(tenants) == 0 {
			fmt.Println("📋 No tenants with custom databases found")
			return nil
		}

		fmt.Printf("🚀 Running migrations on %d tenant(s)...\n", len(tenants))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		successCount := 0
		failCount := 0

		for _, tenant := range tenants {
			fmt.Printf("\n🔄 Migrating tenant: %s\n", tenant.Slug)
			err := service.RunTenantMigrations(ctx, tenant.Slug)
			if err != nil {
				fmt.Printf("❌ Failed: %v\n", err)
				failCount++
			} else {
				fmt.Printf("✅ Success\n")
				successCount++
			}
		}

		fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("📊 Results: %d succeeded, %d failed\n", successCount, failCount)

		return nil
	}

	// Migrate specific tenant
	fmt.Printf("🔄 Running migrations for tenant: %s\n", target)
	err := service.RunTenantMigrations(ctx, target)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	fmt.Println("✅ Migrations completed successfully")

	return nil
}

// maskDatabaseURL masks sensitive parts of database URL
func maskDatabaseURL(url string) string {
	// Simple masking - hide password
	// postgres://user:password@host:port/db -> postgres://user:***@host:port/db
	if len(url) < 20 {
		return "***"
	}
	return url[:15] + "***" + url[len(url)-20:]
}
