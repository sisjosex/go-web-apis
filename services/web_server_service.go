package services

import (
	"josex/web/config"
	"josex/web/interfaces"
	"josex/web/validators"
	"runtime"

	"github.com/gin-gonic/gin"
)

type webServerService struct {
	Server *gin.Engine
}

func NewWebServerService() *webServerService {
	return &webServerService{
		Server: gin.New(),
	}
}

func (ws *webServerService) Initialize(dbService *interfaces.DatabaseService) {

	// Register validations
	validators.RegisterValidations()

	// Set number of CPU
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Set mode
	gin.SetMode(config.AppMode)

	// Setup routes
	ws.Server.SetTrustedProxies(nil)
}

func (ws *webServerService) Start() {
	ws.Server.Run(config.ApplicationHost + ":" + config.ApplicationPort)
	ws.Server.Run()
}
