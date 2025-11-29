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
)
