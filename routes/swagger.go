//go:build !noswagger

package routes

import (
	_ "josex/web/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// registerSwagger mounts the API docs at /swagger. The production image builds with
// -tags noswagger (swagger_off.go): the UI is ~9 MB of the binary and maps the whole API.
func registerSwagger(r *gin.Engine) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
