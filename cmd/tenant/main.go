// @title Tenant API - Business Operations
// @version 1.0
// @description Business API for tracking, invoicing, and operations
// @host localhost:9080
// @BasePath /api/v1
package main

import (
	"context"
	"josex/web/config"
	"josex/web/modules/core/services"
	"josex/web/routes"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func init() {
	// Explicitly set which .env file to load
	os.Setenv("ENV_FILE", ".env.tenant")
}

func main() {
	// Load translations from all enabled modules
	languages := []string{"en", "es"}
	enabledModules := config.GetConfig().Core.EnabledModules
	services.LoadAllTranslations(languages, enabledModules)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Database service
	dbService := services.NewDatabaseService()
	go dbService.InitDatabase(ctx)

	// Web server
	webServer := services.NewWebServerService()
	webServer.Initialize()
	routes.SetupRoutes(webServer.Server, dbService)

	// Handle termination signals
	go func() {
		<-signalChan
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		dbService.CloseDatabase(shutdownCtx)
	}()

	webServer.Start(signalChan)
}
