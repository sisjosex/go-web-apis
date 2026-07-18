package models

// Tenant roles are per-tenant memberships stored in tenant_users.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// IsPrivilegedTenantRole reports whether a tenant role bypasses permission
// checks. It is the single source of truth for that bypass set — HasPermission
// and route guards both derive from it.
func IsPrivilegedTenantRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin:
		return true
	}
	return false
}
