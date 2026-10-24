//go:build !noswagger

package routes

import (
	"net/http"

	coreAPI "josex/web/modules/core/api"

	"github.com/gin-gonic/gin"
)

// docsPage renders /openapi.json with Stoplight Elements, from its CDN: nothing of it is in the binary.
const docsPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Taypi API</title>
  <script src="https://unpkg.com/@stoplight/elements@9.0.15/web-components.min.js"></script>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements@9.0.15/styles.min.css">
</head>
<body style="height:100vh">
  <elements-api apiDescriptionUrl="/openapi.json" router="hash" layout="sidebar"></elements-api>
</body>
</html>`

// registerSwagger mounts the contract (INFRA-021) at /openapi.json and its reader at /docs. The
// production image builds with -tags noswagger (swagger_off.go): the contract maps the whole API.
func registerSwagger(r *gin.Engine) {
	r.GET("/openapi.json", func(c *gin.Context) {
		spec, err := coreAPI.JSON()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, "application/json", spec)
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docsPage))
	})
}
