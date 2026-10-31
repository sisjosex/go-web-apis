package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	coreModels "josex/web/modules/core/models"
	tenancyMW "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"
	"josex/web/modules/users/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes registers user management routes scoped to a tenant.
// Callers must pass the tenant middleware (TenantMiddlewareFromHeader when tenancy
// is enabled, or a no-op handler otherwise).
func RegisterUserRoutes(
	router *gin.RouterGroup,
	userController *controllers.UserController,
	userAuditController *controllers.UserAuditController,
	jwtService services.JWTService,
	tenantMiddleware gin.HandlerFunc,
) {
	userRoutes := router.Group("/users")
	userRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	userRoutes.Use(tenantMiddleware)
	userRoutes.Use(tenancyMW.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin, coreModels.SystemRoleSuperAdmin))
	{
		api.Get(userRoutes, "", userController.ListUsers, api.Doc{Summary: "List the workspace's users, paged and filtered", Errors: []int{400, 401}})
		api.Get(userRoutes, "/stats", userController.GetStats, api.Doc{Summary: "Count the workspace's users by status", Errors: []int{400, 401}})
		api.Get(userRoutes, "/audit", userAuditController.ListAudit, api.Doc{Summary: "List user-management audit entries", Errors: []int{401, 500}})
		api.Post(userRoutes, "", userController.Create, api.Doc{Summary: "Create a user and add them to the workspace", Status: 201, Errors: []int{400, 401, 403, 409}})
		api.Get(userRoutes, "/{id}", userController.GetUserById, api.Doc{Summary: "Get a user of the workspace", Errors: []int{400, 401, 404}})
		api.Put(userRoutes, "/{id}", userController.Update, api.Doc{Summary: "Update a user", Errors: []int{400, 401, 404, 409}})
		api.Delete(userRoutes, "/{id}", userController.SoftDeleteUser, api.Doc{Summary: "Soft-delete a user and end their sessions", Errors: []int{400, 401, 403, 404}})
		api.Post(userRoutes, "/{id}/invitation", userController.ResendInvitation, api.Doc{Summary: "Send the invitation again", Description: "To someone of this workspace who never signed in", Status: 202, Errors: []int{400, 401, 404, 409}})
		api.Post(userRoutes, "/{id}/reset-password", userController.ResetPassword, api.Doc{Summary: "Email the user a password reset link", Description: "Answers 200 when the account has none to send, so it never tells whether an account exists.", Errors: []int{400, 401, 500}})
	}
}
