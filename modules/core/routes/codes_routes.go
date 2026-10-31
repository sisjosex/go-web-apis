package routes

import (
	"josex/web/modules/core/api"
	coreControllers "josex/web/modules/core/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterCodesRoutes mounts the barcode a browser prints on a label (CORE-003).
func RegisterCodesRoutes(group *gin.RouterGroup, controller *coreControllers.CodesController, auth, tenant gin.HandlerFunc) {
	api.Get(group, "/codes/barcode/{symbology}", controller.Barcode, api.Doc{Summary: "Draw a barcode", Description: "An SVG of ean13 or code128 data, cacheable for a year", Errors: []int{400}}, auth, tenant)
}
