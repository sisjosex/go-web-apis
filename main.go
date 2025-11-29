// @title API REST
// @version 1.0
// @description Swagger Documentation for API REST
// @host localhost:8080
// @BasePath /api/v1
package main

import (
	"context"
	"josex/web/modules/core/services"
	"josex/web/routes"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// main is the entry point of the application
// It initializes the database connection and starts the web server
// It also closes the database connection when the application is done
// @return void

func main() {

	languages := []string{"en", "es"} // Idiomas soportados
	services.LoadAllTranslations(languages)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Cancelar al final

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Servicio de base de datos
	dbService := services.NewDatabaseService()
	go dbService.InitDatabase(ctx) // Iniciar la conexión de la base de datos en una goroutine

	// Servicio web
	webServer := services.NewWebServerService()
	webServer.Initialize()
	routes.SetupRoutes(webServer.Server, dbService)

	// Manejar señales de terminación
	go func() {
		<-signalChan
		cancel() // Cancela el contexto cuando se recibe la señal de terminación

		// Cerrar la base de datos con un timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		dbService.CloseDatabase(shutdownCtx) // Cerrar la conexión de la base de datos
	}()

	// Iniciar el servidor (bloqueante hasta recibir señal)
	webServer.Start(signalChan)
}
