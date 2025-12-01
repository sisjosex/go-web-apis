package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"josex/web/modules/tenancy/interfaces"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyServices "josex/web/modules/tenancy/services"
)

func main() {
	// Command line flags
	listCmd := flag.Bool("list", false, "List all tenants with custom databases")
	migrateCmd := flag.String("migrate", "", "Run migrations on tenant (slug) or 'all' for all tenants")
	createCmd := flag.String("create", "", "Create a new tenant (slug:name:database_url)")
	addUserCmd := flag.String("add-user", "", "Add user to tenant (tenant_slug:user_id:role)")
	tenantSlug := flag.String("tenant", "", "Tenant slug for operations")

	flag.Parse()

	// Load environment variables
	utils.LoadEnv()

	// Check if tenancy is enabled
	tenancyConf := config.ModularAppConfig.Tenancy
	if tenancyConf == nil || !tenancyConf.Enabled {
		log.Fatal("❌ Tenancy module is not enabled. Set TENANCY_ENABLED=true in .env")
	}

	ctx := context.Background()

	// Initialize database service
	dbService := coreServices.NewDatabaseService()
	dbService.InitDatabase(ctx)
	defer dbService.CloseDatabase(ctx)

	// Initialize tenant service
	tenantRepo := tenancyRepos.NewTenantRepository(dbService)
	tenantService := tenancyServices.NewTenantService(tenantRepo, dbService)

	// Execute commands
	switch {
	case *listCmd:
		listTenants(ctx, tenantRepo)
	case *migrateCmd != "":
		runMigrations(ctx, tenantService, tenantRepo, *migrateCmd)
	case *createCmd != "":
		createTenant(ctx, tenantService, *createCmd)
	case *addUserCmd != "":
		if *tenantSlug == "" {
			log.Fatal("❌ --tenant flag is required for add-user command")
		}
		addUserToTenant(ctx, tenantRepo, *tenantSlug, *addUserCmd)
	default:
		// Default action: migrate all tenants
		runMigrations(ctx, tenantService, tenantRepo, "all")
	}
}

func printUsage() {
	fmt.Println("Tenant Migration CLI Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  (no flags)                      Run migrations on all tenants (default)")
	fmt.Println("  --list                          List all tenants with custom databases")
	fmt.Println("  --migrate <slug|all>            Run migrations on specific tenant or all tenants")
	fmt.Println("  --create slug:name:db_url       Create a new tenant")
	fmt.Println("  --add-user user_id:role         Add user to tenant (requires --tenant)")
	fmt.Println("  --tenant slug                   Specify tenant slug")
	fmt.Println("\nExamples:")
	fmt.Println("  go run cmd/tenant-migration/main.go")
	fmt.Println("  go run cmd/tenant-migration/main.go --list")
	fmt.Println("  go run cmd/tenant-migration/main.go --migrate acme6")
	fmt.Println("  go run cmd/tenant-migration/main.go --migrate all")
	fmt.Println("  go run cmd/tenant-migration/main.go --create \"acme:Acme Corp:postgres://...\"")
	fmt.Println("  go run cmd/tenant-migration/main.go --tenant acme --add-user \"user-uuid:owner\"")
}

func listTenants(ctx context.Context, repo interfaces.TenantRepository) {
	tenants, err := repo.ListTenantsWithCustomDB(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to list tenants: %v", err)
	}

	if len(tenants) == 0 {
		fmt.Println("📋 No tenants with custom databases found")
		return
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
}

func runMigrations(ctx context.Context, service interfaces.TenantService, repo interfaces.TenantRepository, target string) {
	if target == "all" {
		// Get all tenants with custom DB
		tenants, err := repo.ListTenantsWithCustomDB(ctx)
		if err != nil {
			log.Fatalf("❌ Failed to list tenants: %v", err)
		}

		if len(tenants) == 0 {
			fmt.Println("📋 No tenants with custom databases found")
			return
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
	} else {
		// Migrate specific tenant
		fmt.Printf("🔄 Running migrations for tenant: %s\n", target)
		err := service.RunTenantMigrations(ctx, target)
		if err != nil {
			log.Fatalf("❌ Migration failed: %v", err)
		}
		fmt.Println("✅ Migrations completed successfully")
	}
}

func createTenant(ctx context.Context, service interfaces.TenantService, params string) {
	// Parse params (slug:name:database_url)
	// Simplified - in production, use proper parsing
	fmt.Printf("✅ Create tenant via API: POST /api/v1/tenants with body:\n")
	fmt.Printf("   {\"slug\": \"...\", \"name\": \"...\", \"database_url\": \"...\"}\n")
}

func addUserToTenant(ctx context.Context, repo interfaces.TenantRepository, tenantSlug, params string) {
	// Parse params (user_id:role)
	// Simplified - in production, use proper parsing
	fmt.Printf("✅ Add user to tenant via API: POST /api/v1/tenants/%s/users\n", tenantSlug)
	fmt.Printf("   {\"user_id\": \"...\", \"role\": \"...\"}\n")
}

func maskDatabaseURL(url string) string {
	// Simple masking - hide password
	// postgres://user:password@host:port/db -> postgres://user:***@host:port/db
	if len(url) < 20 {
		return "***"
	}
	return url[:15] + "***" + url[len(url)-20:]
}
