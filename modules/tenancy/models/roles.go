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
	// RoleDriver drives a route on the mobile app; which driver record the account belongs to is
	// tracking.drivers.user_id, never a column here (TRACK-006 D1).
	RoleDriver = "driver"
)

// IsMobileOnlyTenantRole reports whether an access level may only sign in through the mobile app.
// The web tenant middleware refuses these outright, off the access row it has already fetched.
func IsMobileOnlyTenantRole(role string) bool {
	switch role {
	case RolePortal, RoleDriver:
		return true
	}
	return false
}

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
