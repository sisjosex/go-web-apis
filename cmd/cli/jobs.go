package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"josex/web/config"
	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingJobs "josex/web/modules/tracking/jobs"
)

// CmdJobs handles the `jobs` (alias: j) subcommand — a scheduled job run once, by hand.
//
// The scheduler role fires these on its own (INFRA-001); the CLI runs the same pass on demand: it
// reports and exits non-zero if anything failed. Running one twice is safe — each job says in its
// own comment why.
func CmdJobs(args []string) {
	fs := flag.NewFlagSet("jobs", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println("⏰ Scheduled jobs")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli jobs <job>")
		fmt.Println("\nJobs:")
		fmt.Println("  document-alerts   Queue each tenant's digest of compliance documents entering")
		fmt.Println("                    their expiry window; the worker mails it (TRACK-016)")
		fmt.Println("\nRuns once and exits; non-zero if any tenant failed. Safe to run again:")
		fmt.Println("a document is named in exactly one digest however often this runs.")
		fmt.Println("\nRequires: .env.platform with TENANCY_ENABLED=true")
	}

	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}

	switch fs.Arg(0) {
	case "document-alerts":
		if err := runDocumentAlerts(); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("❌ Unknown job: %s\n\n", fs.Arg(0))
		fs.Usage()
		os.Exit(1)
	}
}

// runDocumentAlerts runs the same pass the scheduler fires at 06:00 (INFRA-001 D4): each tenant's due
// documents are claimed and written to its outbox, and the worker mails the digest from there. The CLI
// sends nothing itself, so a mail server that is down costs a retry in the worker, not the digest.
//
// A tenant that fails is reported and the walk continues; the exit code carries whether anything did.
func runDocumentAlerts() error {
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

	failed := 0
	for _, r := range trackingJobs.RunDocumentAlertsPass(ctx, dbService, tenants) {
		switch {
		case r.Err != nil:
			fmt.Printf("⚠️  %s: %v\n", r.Tenant.ID, r.Err)
			failed++
		case r.Claimed > 0:
			fmt.Printf("📬 %s: %d document(s) queued for the digest\n", r.Tenant.ID, r.Claimed)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d tenants failed", failed, len(tenants))
	}
	fmt.Printf("✅ Document alerts complete (%d tenants)\n", len(tenants))
	return nil
}

// jobTenants answers every active tenant, shared ones included (DatabaseURL ""): the walk the worker
// and scheduler make (INFRA-009).
func jobTenants(ctx context.Context, dbService coreServices.DatabaseService) ([]coreJobs.Tenant, error) {
	listed, err := tenancyRepos.NewTenantRepository(dbService).ListTenantDirectory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenants: %w", err)
	}
	tenants := make([]coreJobs.Tenant, 0, len(listed))
	for _, t := range listed {
		tenant := coreJobs.Tenant{ID: t.ID.String(), Slug: t.Slug, Name: t.Name}
		if t.DatabaseURL != nil {
			tenant.DatabaseURL = *t.DatabaseURL
		}
		tenants = append(tenants, tenant)
	}
	return tenants, nil
}
