package models

// System roles are global, signed into the JWT, and independent of any tenant.
const (
	SystemRoleSuperAdmin = "super_admin"
	SystemRoleUser       = "user"
)
