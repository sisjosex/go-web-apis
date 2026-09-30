package services

import (
	"context"
	"josex/web/config"
	"josex/web/modules/core/errors"
	"josex/web/modules/core/validators"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type WebServerService struct {
	Server *gin.Engine
}

func NewWebServerService() *WebServerService {
	// The mode is set before the engine exists: gin.New() itself prints the debug-mode warning.
	gin.SetMode(config.ModularAppConfig.Core.AppMode)
	return &WebServerService{
		Server: gin.New(),
	}
}

func (ws *WebServerService) Initialize() {
	// Registrar validaciones personalizadas
	validators.RegisterValidations()

	// Configurar servidor
	ws.setupServer()
	ws.setupRoutes()

	coreConf := config.ModularAppConfig.Core
	log.Printf("Server running in mode: %s", coreConf.AppMode)
}

func (ws *WebServerService) setupServer() {
	coreConf := config.ModularAppConfig.Core

	// A panicking handler answers 500 instead of dropping the connection (INFRA-004). First, so it
	// wraps everything registered after it; the stack still goes to stderr through gin's writer.
	ws.Server.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, errors.BuildErrorSingle(c, "core.internal-error"))
	}))

	// Configurar proxies de confianza
	ws.Server.SetTrustedProxies([]string{"127.0.0.1"})

	// CORS — origins via ALLOWED_ORIGINS env var (default: http://localhost:3000)
	// If-None-Match in, ETag out: a cached read (TRACK-028's route line) is revalidated by the page itself.
	ws.Server.Use(cors.New(cors.Config{
		AllowOrigins:     coreConf.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept-Language", "X-Tenant-Slug", "If-None-Match"},
		ExposeHeaders:    []string{"Content-Length", "ETag"},
		AllowCredentials: true,
	}))
}

func (ws *WebServerService) setupRoutes() {
	ws.Server.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Server is running"})
	})
}

// Serve listens until ctx is cancelled, then drains in-flight requests for up to 5 s and returns, so
// the caller can stop what the handlers depend on only once no request is using it.
func (ws *WebServerService) Serve(ctx context.Context) {
	coreConf := config.ModularAppConfig.Core
	// Header and idle bounds only (INFRA-004): the realtime role hijacks its sockets through this
	// same server, and Caddy already bounds the client side, so no Read/Write timeout here.
	srv := &http.Server{
		Addr:              coreConf.AppHost + ":" + coreConf.AppPort,
		Handler:           ws.Server,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Server started on %s:%s", coreConf.AppHost, coreConf.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %s", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
