package errors

// Tenant errors
const (
	TenantNotFound                 = "tenant.not-found"
	TenantSlugAlreadyExists        = "tenant.slug.already-exists"
	TenantCreateFailed             = "tenant.create.failed"
	TenantUpdateFailed             = "tenant.update.failed"
	TenantDeleteFailed             = "tenant.delete.failed"
	TenantDatabaseConnectionFailed = "tenant.database.connection-failed"
	TenantInactive                 = "tenant.inactive"
	TenantSuspended                = "tenant.suspended"
)

// Tenant user errors
const (
	TenantUserNotFound      = "tenant.user.not-found"
	TenantUserAlreadyExists = "tenant.user.already-exists"
	TenantUserUnauthorized  = "tenant.user.unauthorized"
	TenantUserInvalidRole   = "tenant.user.invalid-role"
	TenantUserAddFailed     = "tenant.user.add-failed"
	TenantUserRemoveFailed  = "tenant.user.remove-failed"
	TenantUserUpdateFailed  = "tenant.user.update-failed"
)

// Tenant validation errors
const (
	TenantSlugInvalid         = "tenant.slug.invalid"
	TenantSlugRequired        = "tenant.slug.required"
	TenantNameRequired        = "tenant.name.required"
	TenantDatabaseURLInvalid  = "tenant.database-url.invalid"
	TenantLimitReached        = "tenant.limit-reached"
	TenantSelfServiceDisabled = "tenant.self-service-disabled"
)

// Multitenancy disabled error
const (
	MultitenancyDisabled = "multitenancy.disabled"
)
