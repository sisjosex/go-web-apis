package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	"josex/web/modules/tenancy/controllers"
	"josex/web/modules/tenancy/interfaces"
	tenantMiddleware "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

// RegisterRoleRoutes registers tenant role management routes.
// Call this alongside RegisterTenantRoutes and RegisterPermissionRoutes in routes/routes.go.
func RegisterRoleRoutes(
	router *gin.RouterGroup,
	roleController *controllers.RoleController,
	tenantService interfaces.TenantService,
	jwtService services.JWTService,
) {
	auth := authMiddleware.AuthMiddleware(jwtService)

	tenantRoutes := router.Group("/tenants/:tenant_slug")
	tenantRoutes.Use(auth, tenantMiddleware.TenantMiddleware(tenantService))
	{
		// Any tenant member can list roles or view a single role
		api.Get(tenantRoutes, "/roles", roleController.ListRoles, api.Doc{Summary: "List roles for a tenant", Description: "Returns all custom roles defined for the current tenant, including permission count", Errors: []int{401}})
		api.Get(tenantRoutes, "/roles/{role_id}", roleController.GetRole, api.Doc{Summary: "Get a role", Description: "Returns a single role with its full permission code list", Errors: []int{400, 401, 404}})
		api.Get(tenantRoutes, "/roles/{role_id}/users", roleController.ListRoleUsers, api.Doc{Summary: "List users assigned to a role", Description: "Returns the active tenant members who currently hold a given role", Errors: []int{400, 401, 404}})

		// owner/admin only: create, update, delete roles and manage permissions
		adminRoutes := tenantRoutes.Group("")
		adminRoutes.Use(tenantMiddleware.RequireTenantRole(tenancyModels.RoleOwner, tenancyModels.RoleAdmin))
		{
			api.Post(adminRoutes, "/roles", roleController.CreateRole, api.Doc{Summary: "Create a role", Description: "Creates a new custom role for the current tenant", Status: 201, Errors: []int{400, 401, 403, 409}})
			api.Patch(adminRoutes, "/roles/{role_id}", roleController.UpdateRole, api.Doc{Summary: "Update a role", Description: "Updates the name and/or description of a custom tenant role", Errors: []int{400, 401, 403, 404, 409}})
			api.Delete(adminRoutes, "/roles/{role_id}", roleController.DeleteRole, api.Doc{Summary: "Delete a role", Description: "Deletes a custom tenant role; system roles cannot be deleted", Status: 204, Errors: []int{400, 401, 403, 404}})
			api.Put(adminRoutes, "/roles/{role_id}/permissions", roleController.SetRolePermissions, api.Doc{Summary: "Set role permissions", Description: "Replaces all permissions for a role atomically; pass an empty array to clear all permissions", Status: 204, Errors: []int{400, 401, 403, 404}})

			// User role assignment/revocation
			api.Post(adminRoutes, "/users/{user_id}/roles/{role_id}", roleController.AssignUserRole, api.Doc{Summary: "Assign a role to a user", Description: "Assigns a custom role to a tenant user; idempotent if already assigned", Status: 204, Errors: []int{400, 401, 403, 404}})
			api.Delete(adminRoutes, "/users/{user_id}/roles/{role_id}", roleController.RevokeUserRole, api.Doc{Summary: "Revoke a role from a user", Description: "Removes a custom role from a tenant user; idempotent if not assigned", Status: 204, Errors: []int{400, 401, 403}})
		}

		// Any tenant member can list a user's roles
		api.Get(tenantRoutes, "/users/{user_id}/roles", roleController.ListUserRoles, api.Doc{Summary: "List roles assigned to a user", Description: "Returns all custom roles currently assigned to a user within the tenant", Errors: []int{400, 401}})
	}
}
