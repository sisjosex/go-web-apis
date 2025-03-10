// @title API REST
// @version 1.0
// @description Swagger Documentation for API REST
// @host localhost:8080
// @BasePath /api/v1
package main

import (
	"context"
	"fmt"
	"josex/web/routes"
	"josex/web/services"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Cancelar al final

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Servicio de base de datos
	dbService := services.NewDatabaseService()
	go dbService.InitDatabase(ctx) // Iniciar la conexión de la base de datos en una goroutine

	// Servicio web
	server := services.NewWebServerService()
	server.Initialize(&dbService)
	routes.SetupRoutes(server.Server, dbService)

	// Manejar señales de terminación
	go func() {
		<-signalChan
		cancel() // Cancela el contexto cuando se recibe la señal de terminación

		// Cerrar la base de datos con un timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		dbService.CloseDatabase(shutdownCtx) // Cerrar la conexión de la base de datos
	}()

	// Iniciar el servidor
	if err := server.Start(ctx); err != nil {
		fmt.Println("Server error:", err)
	}

	// Al finalizar, cerrar la base de datos con un timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	dbService.CloseDatabase(shutdownCtx) // Cerrar la conexión de la base de datos de forma ordenada
}
