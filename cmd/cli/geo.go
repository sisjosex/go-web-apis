package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/utils"
	geoRepos "josex/web/modules/geo/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CmdGeo handles the `geo` subcommand (INFRA-003). docker/geo/switch.sh and rollback.sh call
// `geo import` after every flip, so geo.places always mirrors the build in current/.
func CmdGeo(args []string) {
	fs := flag.NewFlagSet("geo", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Println("🗺️  Geo data")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli geo import <places.geojsonl>")
		fmt.Println("\nReplaces geo.places with the file's features in one transaction; a search")
		fmt.Println("sees the old set or the new one. Non-zero exit on failure, nothing changed.")
		fmt.Println("\nThe database is chosen by ENV_FILE (default: .env.platform, whose server reads geo.places):")
		fmt.Println("  go run ./cmd/cli geo import docker/geo/data/current/places.geojsonl")
	}

	_ = fs.Parse(args)

	if fs.NArg() != 2 || fs.Arg(0) != "import" {
		fs.Usage()
		os.Exit(1)
	}

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}
	utils.LoadEnv()

	if err := importPlaces(fs.Arg(1)); err != nil {
		fmt.Printf("❌ Import failed: %v\n", err)
		os.Exit(1)
	}
}

// poolOnly hands the repository a pool without the server's DatabaseService, which retries forever
// and migrates — wrong for a one-shot command.
type poolOnly struct{ pool *pgxpool.Pool }

func (p poolOnly) GetPrimaryPool() *pgxpool.Pool { return p.pool }

func importPlaces(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	ctx := context.Background()
	// Core config only, as in migrate.go: the auth config panics without JWT keys.
	pool, err := pgxpool.New(ctx, coreConfig.LoadCoreConfig().DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	start := time.Now()
	count, err := geoRepos.NewPlacesRepository(poolOnly{pool}).Import(ctx, file)
	if err != nil {
		return err
	}
	fmt.Printf("✅ %d places imported in %s\n", count, time.Since(start).Round(time.Second))
	return nil
}
