package errors

// Users module error codes
const (
	// User CRUD errors
	UserCreateFailed      = "user.create.failed"
	UserUpdateFailed      = "user.update.failed"
	UserValidationFailed  = "user.create.validation-failed"
	UserEmailAlreadyInUse = "user.create.email-in-use"

	// User search/retrieval
	UserNotFound          = "user.not-found"
	UserSearchFailed      = "user.search.failed"
	UserGetByIdNotFound   = "user.get-by-id.not-found"
	UserGetByIdEmailFound = "user.get-by-email.not-found"
	UserListFailed        = "user.list.failed"
	UserGetFailed         = "user.get.failed"

	// User deletion
	UserDeleteFailed     = "user.delete.failed"
	UserAlreadyDeleted   = "user.already-deleted"
	UserDeleteNotAllowed = "user.delete.not-allowed"
	UserDeleteLastOwner  = "user.delete.last-owner"

	// Password reset (admin-triggered)
	UserResetPasswordFailed = "user.reset-password.failed"

	// Audit log
	UserAuditListFailed = "user.audit.list-failed"

	// CSV import — per-row validation codes (mapped to the frontend)
	UserImportFirstNameRequired = "users.import.first-name-required"
	UserImportLastNameRequired  = "users.import.last-name-required"
	UserImportEmailRequired     = "users.import.email-required"
	UserImportEmailInvalid      = "users.import.email-invalid"
	UserImportEmailDuplicate    = "users.import.email-duplicate"
	UserImportBirthdayInvalid   = "users.import.birthday-invalid"
	UserImportWebsiteInvalid    = "users.import.website-invalid"
	UserImportImageMissing      = "users.import.image-missing"
	UserImportImageInvalid      = "users.import.image-invalid"
	UserImportCreateFailed      = "users.import.create-failed"
)
