// Tenancy CLI - Manage tenant instances and run tenant migrations
//
// This CLI requires .env.platform with TENANCY_ENABLED=true
//
// Usage:
//
//	go run ./cmd/tenancy-cli -list
//	go run ./cmd/tenancy-cli -migrate all
//	go run ./cmd/tenancy-cli -migrate <slug>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"josex/web/modules/tenancy/interfaces"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyServices "josex/web/modules/tenancy/services"
)

func init() {
	// Set environment BEFORE any config loading happens
	if envFile := os.Getenv("ENV_FILE"); envFile == "" {
		os.Setenv("ENV_FILE", ".env.platform")
		// Note: Don't hardcode ENABLED_MODULES - read from .env.platform
		// This way tenancy-cli respects the platform configuration
	}
}

func main() {
	list := flag.Bool("list", false, "List all tenants with custom databases")
	migrate := flag.String("migrate", "", "Run migrations on tenant (slug) or 'all' for all tenants")
	help := flag.Bool("h", false, "Show help")

	flag.Parse()

	if *help {
		printUsage()
		os.Exit(0)
	}

	// Default action: list all tenants
	if !*list && *migrate == "" {
		*list = true
	}

	if err := handleTenancyOps(*list, *migrate); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
}

func setEnvForTenancy() {
	if envFile := os.Getenv("ENV_FILE"); envFile != "" {
		return
	}
	os.Setenv("ENV_FILE", ".env.platform")
	os.Setenv("ENABLED_MODULES", "core,auth,users,tenancy")
}

func handleTenancyOps(list bool, migrate string) error {
	utils.LoadEnv()

	// Load config (lazy-loaded on first access)
	cfg := config.GetConfig()
	if cfg == nil {
		return fmt.Errorf("failed to load configuration")
	}

	tenancyEnabled := utils.GetEnv("TENANCY_ENABLED", "false")
	if tenancyEnabled != "true" {
		return fmt.Errorf("tenancy not enabled. Set TENANCY_ENABLED=true in .env.platform")
	}

	ctx := context.Background()
	dbService := coreServices.NewDatabaseService()
	dbService.InitDatabase(ctx)
	defer dbService.CloseDatabase(ctx)

	tenantRepo := tenancyRepos.NewTenantRepository(dbService)
	tenantService := tenancyServices.NewTenantService(tenantRepo, dbService)

	switch {
	case list:
		return listTenants(ctx, tenantRepo)
	case migrate != "":
		return runMigrations(ctx, tenantService, tenantRepo, migrate)
	default:
		return nil
	}
}

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

func runMigrations(ctx context.Context, service interfaces.TenantService, repo interfaces.TenantRepository, target string) error {
	if target == "all" {
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

	fmt.Printf("🔄 Running migrations for tenant: %s\n", target)
	err := service.RunTenantMigrations(ctx, target)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	fmt.Println("✅ Migrations completed successfully")

	return nil
}

func maskDatabaseURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	return url[:15] + "***" + url[len(url)-20:]
}

func printUsage() {
	fmt.Println("🏢 Tenancy Manager - Manage tenant instances and migrations")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/tenancy-cli [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -list               List all tenants with custom databases (default)")
	fmt.Println("  -migrate <slug|all> Run migrations on specific tenant or all tenants")
	fmt.Println("  -h                  Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  go run ./cmd/tenancy-cli")
	fmt.Println("  go run ./cmd/tenancy-cli -list")
	fmt.Println("  go run ./cmd/tenancy-cli -migrate all")
	fmt.Println("  go run ./cmd/tenancy-cli -migrate acme")
	fmt.Println()
	fmt.Println("Requirements:")
	fmt.Println("  - .env.platform must exist with TENANCY_ENABLED=true")
	fmt.Println("  - Database must be initialized and running")
	fmt.Println()
}
