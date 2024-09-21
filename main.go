package main

import (
	"josex/web/interfaces"
	"josex/web/routes"
	"josex/web/services"
)

// Init database connection
func InitDatabaseConnection() interfaces.DatabaseService {
	dbService := services.NewDatabaseService()
	dbService.InitDatabase()
	return dbService
}

// Start web server
func StartWebServer(dbService interfaces.DatabaseService) {

	webServerService := services.NewWebServerService()
	webServerService.Initialize(&dbService)
	routes.SetupRoutes(webServerService.Server, dbService)
	webServerService.Start()
}

// Main function
func main() {
	dbService := InitDatabaseConnection()
	StartWebServer(dbService)

	dbService.CloseDatabase()
}
