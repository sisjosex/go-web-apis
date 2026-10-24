package routes

import (
	"josex/web/modules/core/api"
	coreControllers "josex/web/modules/core/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterStorageRoutes mounts the upload tickets (INFRA-007 D1) behind auth and, for a tenant's
// purpose, the tenant chain: a ticket's key is under its owner, so a claim can check ownership from
// the key alone. A public purpose (an avatar, MEDIA-001) is the user's, and its claim needs only auth.
func RegisterStorageRoutes(group *gin.RouterGroup, controller *coreControllers.StorageController, auth, tenant gin.HandlerFunc) {
	api.Post(group, "/storage/uploads", controller.CreateUpload, api.Doc{Summary: "Request an upload ticket", Description: "Signs a 5-minute PUT straight to storage; the declared type and size are part of the signature", Status: 201, Errors: []int{400, 403}}, auth, controller.TenantUnlessPublic(tenant))
	api.Post(group, "/storage/uploads/claim", controller.ClaimUpload, api.Doc{Summary: "Publish an uploaded image", Description: "Checks a public purpose's upload (its size, and bytes that are the signed type) and publishes it at an immutable URL of the media bucket", Errors: []int{403, 400, 404, 422}}, auth)
}
