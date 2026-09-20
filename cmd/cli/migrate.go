package main

import (
	"flag"
	"fmt"
	"os"

	coreConfig "josex/web/modules/core/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
)

// CmdMigrate handles the `migrate` (alias: mig) subcommand.
//
// It runs the modular migrations of ENABLED_MODULES against the ENV_FILE
// database once and exits — non-zero on failure. The server retries forever on
// purpose; a one-shot migrate job must not, or the deploy hangs instead of
// failing.
func CmdMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println("🗄️  Schema Migrator")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli migrate")
		fmt.Println("\nRuns the migrations of every module in ENABLED_MODULES against")
		fmt.Println("DATABASE_URL, then exits. Non-zero exit if any migration fails.")
		fmt.Println("\nThe database is chosen by ENV_FILE (default: .env.platform):")
		fmt.Println("  ENV_FILE=.env.tenant go run ./cmd/cli migrate")
	}

	// ExitOnError: Parse never returns, it exits.
	_ = fs.Parse(args)

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}

	utils.LoadEnv()

	// Core config only, never config.GetConfig(): that loads every module's
	// config, and the auth one panics without JWT_SECRET_KEY. A schema
	// migration needs DATABASE_URL and ENABLED_MODULES, nothing else — .env
	// files for servers that never mint a token have no JWT keys in them.
	coreConf := coreConfig.LoadCoreConfig()

	if err := coreServices.RunModularMigrations(coreConf.DatabaseURL); err != nil {
		fmt.Printf("❌ Migrations failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ All modular migrations completed successfully")
}
