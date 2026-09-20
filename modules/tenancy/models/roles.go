package models

// Tenant roles are per-tenant memberships stored in tenant_users.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	// RoleOrganization sees only the organizations it is a member of (TRACK-015 D1); the scope
	// lives in tracking.organization_members, not here.
	RoleOrganization = "organization"
	// RolePortal is a guardian on the mobile app; the web tenant middleware refuses it (D2).
	RolePortal = "portal"
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
