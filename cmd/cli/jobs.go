package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyInterfaces "josex/web/modules/tenancy/interfaces"
	tenancyModels "josex/web/modules/tenancy/models"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	trackingModels "josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
)

// CmdJobs handles the `jobs` (alias: j) subcommand — the scheduled work that has no scheduler yet.
//
// Until INFRA-001 brings an outbox and a scheduler role, a job is a one-shot process cron runs: it
// does its pass, reports, and exits non-zero if anything failed. Nothing here is a daemon, nothing
// holds a lock, and running one twice is safe — each job says in its own comment why.
func CmdJobs(args []string) {
	fs := flag.NewFlagSet("jobs", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println("⏰ Scheduled jobs")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli jobs <job>")
		fmt.Println("\nJobs:")
		fmt.Println("  document-alerts   Email each tenant's owners and admins about compliance")
		fmt.Println("                    documents entering their expiry window (TRACK-016)")
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

// runDocumentAlerts walks every tenant with its own database and sends one digest per tenant.
//
// The recipients come from the platform database (tenant_users is not a tenant table); the documents
// come from that tenant's own pool. A tenant that fails is reported and the walk continues — one
// unreachable database must not cost every other tenant its digest — and the exit code carries
// whether anything failed.
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

	tenantRepo := tenancyRepos.NewTenantRepository(dbService)
	emailService := coreServices.NewEmailService()

	tenants, err := tenantRepo.ListTenantsWithCustomDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	failed := 0
	for _, tenant := range tenants {
		if tenant.DatabaseURL == nil || *tenant.DatabaseURL == "" {
			continue
		}
		sent, err := alertOneTenant(ctx, dbService, tenantRepo, emailService, tenant)
		if err != nil {
			fmt.Printf("⚠️  %s: %v\n", tenant.Slug, err)
			failed++
			continue
		}
		if sent > 0 {
			fmt.Printf("📧 %s: %d document(s) reported\n", tenant.Slug, sent)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d tenants failed", failed, len(tenants))
	}
	fmt.Printf("✅ Document alerts complete (%d tenants)\n", len(tenants))
	return nil
}

// alertOneTenant claims one tenant's due documents and mails them. It answers with how many were
// claimed, which is zero on every run after the first until something new enters a window.
func alertOneTenant(
	ctx context.Context,
	dbService coreServices.DatabaseService,
	tenantRepo tenancyInterfaces.TenantRepository,
	emailService coreServices.EmailService,
	tenant *tenancyModels.Tenant,
) (int, error) {
	// Claim first: the UPDATE … RETURNING is what makes "exactly one alert" true, so nothing is
	// claimed that the tenant has no recipients for — a tenant with no owner is a misconfiguration
	// to fix, not a reason to re-send later.
	recipients, err := tenantRepo.ListTenantAdminEmails(ctx, tenant.ID)
	if err != nil {
		return 0, fmt.Errorf("recipients: %w", err)
	}
	if len(recipients) == 0 {
		return 0, nil
	}

	// The same key the tenant middleware sets on a request: the shared DatabaseService resolves
	// every query in this context against that tenant's pool, and caches it like any other.
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, *tenant.DatabaseURL)
	if _, err := dbService.GetPoolForTenant(tenantCtx, *tenant.DatabaseURL); err != nil {
		return 0, fmt.Errorf("tenant database: %w", err)
	}

	alerts, err := trackingRepos.NewTrackingRepository(dbService).RaiseDocumentAlerts(tenantCtx, tenant.ID)
	if err != nil {
		return 0, fmt.Errorf("raise alerts: %w", err)
	}
	if len(alerts) == 0 {
		return 0, nil
	}

	sendDocumentDigest(emailService, tenant, recipients, alerts)
	return len(alerts), nil
}

// sendDocumentDigest mails one digest per recipient. It is deliberately not a goroutine: a one-shot
// job that exits while its sends are in flight sends nothing.
func sendDocumentDigest(
	emailService coreServices.EmailService,
	tenant *tenancyModels.Tenant,
	recipients []*tenancyModels.TenantAdminEmail,
	alerts []*trackingModels.DocumentAlert,
) {
	appConf := config.ModularAppConfig.Core
	subject := fmt.Sprintf("%d document(s) need attention in %s", len(alerts), tenant.Name)
	templatePath := coreServices.GetTemplatePath("tracking", "document-expiry-digest.html")

	data := map[string]interface{}{
		"Title":         subject,
		"TenantName":    tenant.Name,
		"Description":   "These compliance documents have entered their warning window. A document is named here once; renew it and it will be watched again.",
		"Documents":     digestLines(alerts),
		"ButtonText":    "Open documents",
		"DocumentsURL":  fmt.Sprintf("%s/tracking/documents", appConf.FrontendURL),
		"SignOff":       fmt.Sprintf("The %s team", appConf.AppName),
		"AutomatedNote": "This is an automated message. Please do not reply.",
	}

	for _, recipient := range recipients {
		if err := emailService.SendEmail(recipient.Email, subject, templatePath, data); err != nil {
			fmt.Printf("⚠️  %s: could not mail %s: %v\n", tenant.Slug, recipient.Email, err)
		}
	}
}

// digestLines turns the claimed rows into what the template renders, so the template holds no logic.
func digestLines(alerts []*trackingModels.DocumentAlert) []map[string]string {
	lines := make([]map[string]string, 0, len(alerts))
	for _, alert := range alerts {
		subject := "—"
		if alert.SubjectName != nil {
			subject = *alert.SubjectName
		}
		number := ""
		if alert.Number != nil {
			number = *alert.Number
		}
		lines = append(lines, map[string]string{
			"Subject":   subject,
			"Type":      alert.TypeName,
			"Number":    number,
			"ExpiresOn": alert.ExpiresOn.Format(time.DateOnly),
			"When":      expiryPhrase(alert.DaysLeft),
		})
	}
	return lines
}

// expiryPhrase says what the number of days means, so the reader does not do the arithmetic.
func expiryPhrase(daysLeft int32) string {
	switch {
	case daysLeft < 0:
		return fmt.Sprintf("expired %d day(s) ago", -daysLeft)
	case daysLeft == 0:
		return "expires today"
	default:
		return fmt.Sprintf("expires in %d day(s)", daysLeft)
	}
}
