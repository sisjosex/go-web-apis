//go:build noswagger

package routes

import "github.com/gin-gonic/gin"

// registerSwagger mounts nothing: see swagger.go.
func registerSwagger(*gin.Engine) {}
