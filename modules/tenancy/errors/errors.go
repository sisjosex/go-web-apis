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
	TenantUserNotFound                = "tenant.user.not-found"
	TenantUserAlreadyExists           = "tenant.user.already-exists"
	TenantUserAlreadyRemoved          = "tenant.user.already-removed"
	TenantUserUnauthorized            = "tenant.user.unauthorized"
	TenantUserInvalidRole             = "tenant.user.invalid-role"
	TenantUserAddFailed               = "tenant.user.add-failed"
	TenantUserRemoveFailed            = "tenant.user.remove-failed"
	TenantUserUpdateFailed            = "tenant.user.update-failed"
	TenantUserInsufficientPermissions = "tenant.user.insufficient-permissions"
	TenantUserNotAuthorized           = "tenant.user.not-authorized"
	TenantUserCannotRemoveOwner       = "tenant.user.cannot-remove-owner"
	TenantUserLastOwner               = "tenant.user.last-owner"
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

// Permission errors
const (
	PermissionDenied = "tenant.permission.denied"
)

// Module errors
const (
	ModuleNotFound     = "tenant.module.not-found"
	ModuleNotEnabled   = "tenant.module.not-enabled"
	ModuleCodeRequired = "tenant.module.code-required"
	ModuleUpdateFailed = "tenant.module.update-failed"
)

// Role errors
const (
	RoleNotFound         = "tenant.role.not-found"
	RoleDuplicateName    = "tenant.role.create.duplicate-name"
	RoleUpdateSystemRole = "tenant.role.update.system-role"
	RoleDeleteSystemRole = "tenant.role.delete.system-role"
	RoleValidationFailed = "tenant.role.validation-failed"
)
