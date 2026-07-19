// @title Go Web API
// @version 1.0
// @description Modular API server. Run in platform or tenant mode via -mode flag.
// @host localhost:8080
// @BasePath /api/v1
package main

import (
	"context"
	"flag"
	"fmt"
	"josex/web/config"
	"josex/web/modules/core/services"
	"josex/web/routes"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mode := flag.String("mode", "", "Server mode: platform or tenant")
	flag.Parse()

	// Resolve mode: flag > ENV_FILE already set > default to platform
	if *mode == "" {
		switch os.Getenv("ENV_FILE") {
		case ".env.tenant":
			*mode = "tenant"
		default:
			*mode = "platform"
		}
	}

	switch *mode {
	case "platform":
		os.Setenv("ENV_FILE", ".env.platform")
	case "tenant":
		os.Setenv("ENV_FILE", ".env.tenant")
	default:
		fmt.Fprintf(os.Stderr, "❌ Unknown mode %q — use -mode=platform or -mode=tenant\n", *mode)
		os.Exit(1)
	}

	fmt.Printf("🚀 Starting server in %s mode\n", *mode)

	// Initialize the global modular config (reads ENV_FILE) before anything uses it.
	config.GetConfig()

	languages := []string{"en", "es"}
	services.LoadAllTranslations(languages)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	dbService := services.NewDatabaseService()
	go dbService.InitDatabase(ctx)

	webServer := services.NewWebServerService()
	webServer.Initialize()
	routes.SetupRoutes(webServer.Server, dbService)

	go func() {
		<-signalChan
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		dbService.CloseDatabase(shutdownCtx)
	}()

	webServer.Start(signalChan)
}
