package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"josex/web/modules/tenancy/interfaces"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	tenancyServices "josex/web/modules/tenancy/services"

	"github.com/google/uuid"
)

// CmdTenant handles the `tenant` (alias: t) subcommand
func CmdTenant(args []string) {
	fs := flag.NewFlagSet("tenant", flag.ExitOnError)
	listFlag := fs.Bool("list", false, "List all tenants with custom databases")
	migrateFlag := fs.String("migrate", "", "Run migrations: tenant slug or 'all'")
	gpsDeviceFlag := fs.String("gps-device", "", "Issue a GPS device token in this tenant (with -vehicle)")
	vehicleFlag := fs.String("vehicle", "", "The vehicle id the GPS device posts for")

	fs.Usage = func() {
		fmt.Println("🏢 Tenant Manager")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli t [flags]")
		fmt.Println("\nFlags:")
		fmt.Println("  -list                 List all tenants with custom databases (default)")
		fmt.Println("  -migrate <slug|all>   Run migrations on a tenant or all tenants")
		fmt.Println("  -gps-device <slug> -vehicle <uuid>")
		fmt.Println("                        Issue the vehicle's GPS device token (rotates the old one)")
		fmt.Println("\nExamples:")
		fmt.Println("  go run ./cmd/cli t -list")
		fmt.Println("  go run ./cmd/cli t -migrate acme")
		fmt.Println("  go run ./cmd/cli t -migrate all")
		fmt.Println("  go run ./cmd/cli t -gps-device acme -vehicle 0b1c…")
		fmt.Println("\nRequires: .env.platform with TENANCY_ENABLED=true")
	}

	fs.Parse(args)

	// Default action when no flag is given
	if !*listFlag && *migrateFlag == "" && *gpsDeviceFlag == "" {
		*listFlag = true
	}

	// Ensure platform env is loaded
	if os.Getenv("ENV_FILE") == "" {
		os.Setenv("ENV_FILE", ".env.platform")
		os.Setenv("ENABLED_MODULES", "core,auth,tenancy,users")
	}

	if err := runTenantOps(*listFlag, *migrateFlag, *gpsDeviceFlag, *vehicleFlag); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

func runTenantOps(list bool, migrate, gpsDevice, vehicle string) error {
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
	case gpsDevice != "":
		return issueGPSDevice(ctx, tenantRepo, tenancyRepos.NewGPSDeviceRepository(dbService), gpsDevice, vehicle)
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

// issueGPSDevice prints a new device token for the vehicle, once (TRACK-010): the tenant id and 32
// random bytes; only its sha256 is stored, so a lost token is reissued, never recovered.
func issueGPSDevice(ctx context.Context, tenants interfaces.TenantRepository, devices interfaces.GPSDeviceRepository, slug, vehicle string) error {
	vehicleID, err := uuid.Parse(vehicle)
	if err != nil {
		return fmt.Errorf("-vehicle must be the vehicle's uuid: %w", err)
	}
	tenant, err := tenants.GetTenantBySlug(ctx, slug)
	if err != nil {
		return fmt.Errorf("tenant %s: %w", slug, err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	token := tenancyRepos.GPSDeviceToken(tenant.ID, secret)
	_, hash, _ := tenancyRepos.ParseGPSDeviceToken(token)
	if err := devices.Issue(ctx, tenant.ID, vehicleID, hash); err != nil {
		return fmt.Errorf("issue gps device: %w", err)
	}
	fmt.Printf("🔑 GPS device token for vehicle %s in %s (shown once; send it as X-Device-Token):\n%s\n", vehicleID, slug, token)
	return nil
}
