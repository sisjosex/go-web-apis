package errors

// Auth module error codes
const (
	// Authentication errors
	UserLoginFailed              = "user.login.invalid-credentials2"
	UserLoginValidationFailed    = "user.login.validation-failed"
	UserLoginNotFound            = "user.login.not-found"
	UserLogoutValidationFailed   = "user.logout.validation-failed"
	UserRegisterFailed           = "user.register.failed"
	UserRegisterValidationFailed = "user.register.validation-failed"

	// Profile errors
	UserProfileGetFailed        = "user.profile.get.failed"
	UserProfileUpdateFailed     = "user.profile.update.failed"
	UserProfileValidationFailed = "user.profile.validation-failed"

	// Email verification
	UserRequestEmailError       = "user.email.request.failed"
	UserEmailVerification       = "user.email.verification.failed"
	UserChangeEmailSendingError = "user.change-email.sending-email-failed"

	// Password management
	UserChangePasswordError             = "user.change-password.failed"
	UserPasswordResetError              = "user.password-reset.failed"
	UserForgorPasswordEmailSendingError = "user.forgot-password.sending-email-failed"

	// Token errors
	TokenRefreshInvalid       = "token.refresh.invalid"
	TokenRefreshExpired       = "token.refresh.expired"
	TokenRefreshClaimsInvalid = "token.refresh.claims-invalid"

	// Session errors
	SessionInactive         = "session.inactive"
	SessionNotFound         = "session.not-found"
	SessionListFailed       = "session.list-failed"
	SessionLogoutFailed     = "session.logout-failed"
	SessionAlreadyLoggedOut = "session.already-logged-out"
	SessionUnauthorized     = "session.unauthorized"

	// OTP errors
	OtpRequestFailed       = "otp.request.failed"
	OtpGenerated           = "otp.request.generated"
	OtpVerifyFailed        = "otp.verify.failed"
	OtpInvalid             = "otp.verify.invalid"
	OtpExpired             = "otp.verify.expired"
	OtpNotFound            = "otp.not-found"
	OtpMaxAttempts         = "otp.verify.max-attempts"
	OtpDestinationInvalid  = "otp.destination.invalid"
	OtpChannelInvalid      = "otp.channel.invalid"
	OtpChannelDisabled     = "otp.channel.disabled"
	OtpCodeInvalid         = "otp.code.invalid"
	PhoneAlreadyRegistered = "phone.already-registered"
)
