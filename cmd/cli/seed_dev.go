package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"

	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/utils"

	"github.com/jackc/pgx/v5"
)

//go:embed seed_dev.sql
var seedDevSQL string

// CmdSeedDev handles the `seed-dev` subcommand: the dev accounts the emulator signs in with,
// written into the ENV_FILE database (default .env.platform). Idempotent; see seed_dev.sql.
func CmdSeedDev(args []string) {
	fs := flag.NewFlagSet("seed-dev", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("🌱 Dev seed")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli seed-dev")
		fmt.Println("\nWrites the dev accounts of cmd/cli/seed_dev.sql into DATABASE_URL (ENV_FILE,")
		fmt.Println("default .env.platform). Safe to run again; does nothing without tenant mi-negocio.")
	}
	_ = fs.Parse(args)

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}
	utils.LoadEnv()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, coreConfig.LoadCoreConfig().DatabaseURL)
	if err != nil {
		fmt.Printf("❌ Connect: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, seedDevSQL); err != nil {
		fmt.Printf("❌ Seed failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Dev seed applied")
}
