package routes

import (
	coreControllers "josex/web/modules/core/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterStorageRoutes mounts the upload tickets (INFRA-007 D1) behind auth and the tenant chain:
// a ticket's key is under the caller's tenant, so a claim can check ownership from the key alone.
func RegisterStorageRoutes(api *gin.RouterGroup, controller *coreControllers.StorageController, auth, tenant gin.HandlerFunc) {
	api.POST("/storage/uploads", auth, tenant, controller.CreateUpload)
}
