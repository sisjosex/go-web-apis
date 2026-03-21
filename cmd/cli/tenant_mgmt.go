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

// CmdTenant handles the `tenant` (alias: t) subcommand
func CmdTenant(args []string) {
	fs := flag.NewFlagSet("tenant", flag.ExitOnError)
	listFlag := fs.Bool("list", false, "List all tenants with custom databases")
	migrateFlag := fs.String("migrate", "", "Run migrations: tenant slug or 'all'")

	fs.Usage = func() {
		fmt.Println("🏢 Tenant Manager")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli t [flags]")
		fmt.Println("\nFlags:")
		fmt.Println("  -list                 List all tenants with custom databases (default)")
		fmt.Println("  -migrate <slug|all>   Run migrations on a tenant or all tenants")
		fmt.Println("\nExamples:")
		fmt.Println("  go run ./cmd/cli t -list")
		fmt.Println("  go run ./cmd/cli t -migrate acme")
		fmt.Println("  go run ./cmd/cli t -migrate all")
		fmt.Println("\nRequires: .env.platform with TENANCY_ENABLED=true")
	}

	fs.Parse(args)

	// Default action when no flag is given
	if !*listFlag && *migrateFlag == "" {
		*listFlag = true
	}

	// Ensure platform env is loaded
	if os.Getenv("ENV_FILE") == "" {
		os.Setenv("ENV_FILE", ".env.platform")
		os.Setenv("ENABLED_MODULES", "core,auth,users,tenancy")
	}

	if err := runTenantOps(*listFlag, *migrateFlag); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

func runTenantOps(list bool, migrate string) error {
	utils.LoadEnv()
	config.GetConfig()

	if utils.GetEnv("TENANCY_ENABLED", "false") != "true" {
		return fmt.Errorf("tenancy is not enabled — set TENANCY_ENABLED=true in .env.platform")
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
	}
	return nil
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

	fmt.Printf("📋 %d tenant(s) with custom databases:\n", len(tenants))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, t := range tenants {
		status := "✅ active"
		if !t.IsActive {
			status = "❌ inactive"
		}
		fmt.Printf("  %-20s %s  %s\n", t.Slug, t.Name, status)
		if t.DatabaseURL != nil {
			fmt.Printf("  %-20s %s\n", "", maskDatabaseURL(*t.DatabaseURL))
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return nil
}

func runMigrations(ctx context.Context, service interfaces.TenantService, repo interfaces.TenantRepository, target string) error {
	if target != "all" {
		fmt.Printf("🔄 Migrating tenant: %s\n", target)
		if err := service.RunTenantMigrations(ctx, target); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		fmt.Println("✅ Done")
		return nil
	}

	tenants, err := repo.ListTenantsWithCustomDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}
	if len(tenants) == 0 {
		fmt.Println("📋 No tenants with custom databases found")
		return nil
	}

	fmt.Printf("🚀 Migrating %d tenant(s)...\n", len(tenants))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	ok, fail := 0, 0
	for _, t := range tenants {
		fmt.Printf("  🔄 %-20s", t.Slug)
		if err := service.RunTenantMigrations(ctx, t.Slug); err != nil {
			fmt.Printf("❌ %v\n", err)
			fail++
		} else {
			fmt.Println("✅")
			ok++
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 %d succeeded, %d failed\n", ok, fail)
	return nil
}

// maskDatabaseURL hides the password in a postgres connection string
func maskDatabaseURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	return url[:15] + "***" + url[len(url)-15:]
}
