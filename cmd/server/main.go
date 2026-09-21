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
	coreRoutes "josex/web/modules/core/routes"
	"josex/web/modules/core/services"
	"josex/web/routes"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Runtime roles (INFRA-001). One binary, one image: a role picks which parts of it this process runs.
const (
	roleAll       = "all"
	roleAPI       = "api"
	roleRealtime  = "realtime"
	roleWorker    = "worker"
	roleScheduler = "scheduler"
)

// modeRoles is what each mode may run (D2): the worker and the scheduler walk every tenant, which
// only the platform database lists; realtime is the tenant's WebSocket gateway (TRACK-010).
var modeRoles = map[string][]string{
	"platform": {roleAll, roleAPI, roleWorker, roleScheduler},
	"tenant":   {roleAll, roleAPI, roleRealtime},
}

func main() {
	mode := flag.String("mode", "", "Server mode: platform or tenant")
	roleFlag := flag.String("role", "", "Runtime role: all, api, realtime, worker or scheduler (default: APP_ROLE, then all)")
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

	// Initialize the global modular config (reads ENV_FILE) before anything uses it.
	config.GetConfig()
	coreConf := config.ModularAppConfig.Core

	// Resolve role: flag > APP_ROLE > all
	role := *roleFlag
	if role == "" {
		role = coreConf.AppRole
	}
	if !allowedRole(*mode, role) {
		fmt.Fprintf(os.Stderr, "❌ Role %q is not available in %s mode — use one of %v\n", role, *mode, modeRoles[*mode])
		os.Exit(1)
	}

	fmt.Printf("🚀 Starting server in %s mode, role %s\n", *mode, role)

	languages := []string{"en", "es"}
	services.LoadAllTranslations(languages)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbService := services.NewDatabaseService()
	go dbService.InitDatabase(ctx)

	valkey, err := services.NewValkeyService(coreConf.RedisURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	background, err := startBackground(ctx, *mode, role, dbService, valkey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	webServer := services.NewWebServerService()
	if role == roleAll || role == roleAPI {
		webServer.Initialize()
		routes.SetupRoutes(webServer.Server, dbService, valkey)
	} else {
		// Every other role serves only the probes: an orchestrator still needs /livez and /readyz, and
		// nothing else may be reachable on a process that is not an API.
		gin.SetMode(coreConf.AppMode)
		coreRoutes.RegisterHealthRoutes(webServer.Server, dbService, valkey, role == roleWorker || role == roleScheduler)
		if role == roleRealtime {
			fmt.Println("ℹ️  realtime: nothing to run until TRACK-010, serving the probes only")
		}
	}

	// Shutdown order: stop taking requests, then stop the background work, then close the pools the
	// two of them were using.
	webServer.Serve(ctx)
	background.Shutdown()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if valkey != nil {
		_ = valkey.Close()
	}
	dbService.CloseDatabase(shutdownCtx)
}

func allowedRole(mode, role string) bool {
	for _, r := range modeRoles[mode] {
		if r == role {
			return true
		}
	}
	return false
}
