package services

import (
	"context"
	"fmt"
	"josex/web/config"
	"josex/web/interfaces"
	"josex/web/validators"
	"log"
	"net/http"
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

func (ws *WebServerService) Initialize(dbService *interfaces.DatabaseService) {
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

	// Configurar proxies de confianza (modificar según entorno)
	ws.Server.SetTrustedProxies([]string{"127.0.0.1"})
}

func (ws *WebServerService) setupRoutes() {
	// Aquí se registrarían las rutas del servidor
	ws.Server.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Server is running"})
	})
}

func (ws *WebServerService) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", config.AppConfig.AppHost, config.AppConfig.AppPort),
		Handler: ws.Server,
	}

	// Goroutine para manejar la señal de apagado
	go func() {
		<-ctx.Done()
		log.Println("Shutting down web server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		} else {
			log.Println("Server stopped")
		}
	}()

	log.Printf("Server started on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
