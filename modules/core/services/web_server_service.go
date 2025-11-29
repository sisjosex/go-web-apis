package services

import (
	"context"
	"josex/web/config"
	"josex/web/modules/core/validators"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type WebServerService struct {
	Server *gin.Engine
}

func NewWebServerService() *WebServerService {
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

	log.Printf("Server running in mode: %s", config.AppConfig.AppMode)
}

func (ws *WebServerService) setupServer() {
	// Configurar modo de Gin
	gin.SetMode(config.AppConfig.AppMode)

	// Configurar proxies de confianza
	ws.Server.SetTrustedProxies([]string{"127.0.0.1"})
}

func (ws *WebServerService) setupRoutes() {
	ws.Server.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Server is running"})
	})
}

func (ws *WebServerService) Start(quit <-chan os.Signal) {
	srv := &http.Server{
		Addr:    config.AppConfig.AppHost + ":" + config.AppConfig.AppPort,
		Handler: ws.Server,
	}

	go func() {
		log.Printf("Server started on %s:%s", config.AppConfig.AppHost, config.AppConfig.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %s", err)
		}
	}()

	// Esperar señal de terminación
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
