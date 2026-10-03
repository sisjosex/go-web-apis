package routes

import (
	coreControllers "josex/web/modules/core/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterStorageRoutes mounts the upload tickets (INFRA-007 D1) behind auth and, for a tenant's
// purpose, the tenant chain: a ticket's key is under its owner, so a claim can check ownership from
// the key alone. A public purpose (an avatar, MEDIA-001) is the user's, and its claim needs only auth.
func RegisterStorageRoutes(api *gin.RouterGroup, controller *coreControllers.StorageController, auth, tenant gin.HandlerFunc) {
	api.POST("/storage/uploads", auth, controller.TenantUnlessPublic(tenant), controller.CreateUpload)
	api.POST("/storage/uploads/claim", auth, controller.ClaimUpload)
}
