package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
)

// CmdTracking handles the `tracking` subcommand — one-off maintenance of the tracking module.
func CmdTracking(args []string) {
	fs := flag.NewFlagSet("tracking", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println("🚌 Tracking maintenance")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli tracking paths backfill")
		fmt.Println("\nCommands:")
		fmt.Println("  paths backfill   Queue the planned line of every route version that has none yet;")
		fmt.Println("                   the worker computes it through Valhalla (TRACK-028)")
		fmt.Println("\nSafe to run again: a version whose stops did not move costs no router call.")
		fmt.Println("\nRequires: .env.platform with TENANCY_ENABLED=true, and a worker running")
	}

	_ = fs.Parse(args)

	if fs.NArg() != 2 || fs.Arg(0) != "paths" || fs.Arg(1) != "backfill" {
		fs.Usage()
		os.Exit(1)
	}

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}

	if err := runPathsBackfill(); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}

// runPathsBackfill writes, in each tenant's outbox, one route.changed row per route holding a version
// with no line; the worker then computes them as it does any change, retries included. A tenant that
// fails is reported and the walk continues.
func runPathsBackfill() error {
	utils.LoadEnv()
	config.GetConfig()

	if utils.GetEnv("TENANCY_ENABLED", "false") != "true" {
		return fmt.Errorf("tenancy is not enabled — set TENANCY_ENABLED=true in .env.platform")
	}

	ctx := context.Background()
	dbService := coreServices.NewDatabaseService()
	dbService.InitDatabase(ctx)
	defer dbService.CloseDatabase(ctx)

	tenants, err := jobTenants(ctx, dbService)
	if err != nil {
		return err
	}

	repo := trackingRepos.NewTrackingRepository(dbService)
	failed := 0
	for _, tenant := range tenants {
		tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
		enqueued, err := repo.RoutePathsBackfill(tenantCtx, uuid.MustParse(tenant.ID))
		switch {
		case err != nil:
			fmt.Printf("⚠️  %s: %v\n", tenant.ID, err)
			failed++
		case enqueued > 0:
			fmt.Printf("🗺️  %s: %d route(s) queued\n", tenant.Slug, enqueued)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d tenants failed", failed, len(tenants))
	}
	fmt.Printf("✅ Route paths backfill queued (%d tenants)\n", len(tenants))
	return nil
}
