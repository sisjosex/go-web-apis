package main

import (
	"josex/web/config"
	"josex/web/routes"
	"josex/web/services"
	"josex/web/validators"

	"github.com/gin-gonic/gin"
)

// Init database connection
func InitDatabaseConnection() {
	services.InitDatabase()
	//defer services.CloseDatabase()
}

// Start web server
func StartWebServer() {
	gin.SetMode(config.AppMode)

	r := gin.New()

	routes.SetupRoutes(r)

	r.SetTrustedProxies(nil)

	r.Run(config.ApplicationHost + ":" + config.ApplicationPort)
}

// Main function
func main() {

	validators.InitValidator()

	InitDatabaseConnection()

	StartWebServer()
}
