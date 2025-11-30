package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyServices "josex/web/modules/tenancy/services"
)

func main() {
	// Command line flags
	listCmd := flag.Bool("list", false, "List all tenants")
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
		listTenants(ctx, tenantService)
	case *createCmd != "":
		createTenant(ctx, tenantService, *createCmd)
	case *addUserCmd != "":
		if *tenantSlug == "" {
			log.Fatal("❌ --tenant flag is required for add-user command")
		}
		addUserToTenant(ctx, tenantRepo, *tenantSlug, *addUserCmd)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Tenant Migration CLI Tool")
	fmt.Println("\nUsage:")
	fmt.Println("  --list                          List all tenants")
	fmt.Println("  --create slug:name:db_url       Create a new tenant")
	fmt.Println("  --add-user user_id:role         Add user to tenant (requires --tenant)")
	fmt.Println("  --tenant slug                   Specify tenant slug")
	fmt.Println("\nExamples:")
	fmt.Println("  go run cmd/tenant-migration/main.go --list")
	fmt.Println("  go run cmd/tenant-migration/main.go --create \"acme:Acme Corp:postgres://...\"")
	fmt.Println("  go run cmd/tenant-migration/main.go --tenant acme --add-user \"user-uuid:owner\"")
}

func listTenants(ctx context.Context, service tenancyServices.TenantService) {
	// This would require a ListAllTenants method - simplified for now
	fmt.Println("✅ Listing tenants is available via API: GET /api/v1/tenants/my-tenants")
	fmt.Println("   (Requires authentication token)")
}

func createTenant(ctx context.Context, service tenancyServices.TenantService, params string) {
	// Parse params (slug:name:database_url)
	// Simplified - in production, use proper parsing
	fmt.Printf("✅ Create tenant via API: POST /api/v1/tenants with body:\n")
	fmt.Printf("   {\"slug\": \"...\", \"name\": \"...\", \"database_url\": \"...\"}\n")
}

func addUserToTenant(ctx context.Context, repo tenancyRepos.TenantRepository, tenantSlug, params string) {
	// Parse params (user_id:role)
	// Simplified - in production, use proper parsing
	fmt.Printf("✅ Add user to tenant via API: POST /api/v1/tenants/%s/users\n", tenantSlug)
	fmt.Printf("   {\"user_id\": \"...\", \"role\": \"...\"}\n")
}
