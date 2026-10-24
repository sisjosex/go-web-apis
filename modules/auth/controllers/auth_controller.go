package controllers

import (
	"context"
	"errors"
	"josex/web/config"
	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models" // Used in Swagger annotations
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ua-parser/uap-go/uaparser"
)

// Ensure coreModels is used (for Swagger annotations)
var _ coreModels.User

type AuthController struct {
	authService  authInterfaces.AuthService
	jwtService   services.JWTService
	emailService coreServices.EmailService
	parser       *uaparser.Parser
	dbService    coreServices.DatabaseService
	// nil while the server has no Google client (APP-016): /login/google answers 503.
	google services.GoogleSignIn
}

func NewAuthController(
	authService authInterfaces.AuthService,
	jwtService services.JWTService,
	emailService coreServices.EmailService,
	agentParser *uaparser.Parser,
	db coreServices.DatabaseService,
) *AuthController {
	controller := &AuthController{
		authService:  authService,
		jwtService:   jwtService,
		emailService: emailService,
		parser:       agentParser,
		dbService:    db,
	}
	if authConf := config.ModularAppConfig.Auth; authConf.EnableGoogleAuth && authConf.GoogleClientId != "" && authConf.GoogleClientSecret != "" {
		controller.google = services.NewGoogleSignIn(authConf.GoogleClientId, authConf.GoogleClientSecret, authConf.GoogleTokenURL)
	}
	return controller
}

// LoginInput is Login's request.
type LoginInput struct {
	JSON api.Body[authModels.LoginUserRequestDto]
}

// Login is POST /login: Allow login based on email authentication, email can be confirmed after login.
func (uc *AuthController) Login(ctx context.Context, in *LoginInput) (*api.Out[*authModels.LoginSuccessResponse], error) {
	c := api.Gin(ctx)
	var loginUserRequest authModels.LoginUserRequestDto

	if err := in.JSON.Into(ctx, &loginUserRequest); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
	}

	declaredClient, err := clientType(c)
	if err != nil {
		return nil, err
	}

	userAgent := c.GetHeader("User-Agent")

	var deviceInfo, deviceOs string

	// Parse user agent only if parser is available
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family + " " + client.Os.Major)
	}

	loginUser := authModels.LoginUserDto{
		Email:    loginUserRequest.Email,
		Password: loginUserRequest.Password,
		DeviceId: loginUserRequest.DeviceId,
	}

	loginUser.IpAddress = utils.GetClientIp(c)
	loginUser.DeviceInfo = deviceInfo
	loginUser.DeviceOs = deviceOs

	// Set browser info only if parser is available
	var browser string
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		browser = strings.TrimSpace(client.UserAgent.Family + " " + client.UserAgent.Major)
	}
	loginUser.Browser = browser
	loginUser.UserAgent = userAgent
	loginUser.ClientType = declaredClient

	sessionUser, err := uc.authService.LoginUser(c.Request.Context(), loginUser)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(&authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshtoken,
	})
}

// LoginFacebookInput is LoginFacebook's request.
type LoginFacebookInput struct {
	JSON api.Body[authModels.LoginExternalRequestDto]
}

// LoginFacebook is POST /login/facebook: Allow login based on Facebook authentication.
func (uc *AuthController) LoginFacebook(ctx context.Context, in *LoginFacebookInput) (*api.Out[*authModels.LoginSuccessResponse], error) {
	c := api.Gin(ctx)
	var loginExternalRequestDto authModels.LoginExternalRequestDto

	if err := in.JSON.Into(ctx, &loginExternalRequestDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
	}

	return uc.loginExternal(c, authModels.LoginExternalDto{
		AuthProviderName: "facebook",
		AuthProviderId:   loginExternalRequestDto.AuthProviderId,
		DeviceId:         loginExternalRequestDto.DeviceId,
		FirstName:        loginExternalRequestDto.FirstName,
		LastName:         loginExternalRequestDto.LastName,
		Email:            loginExternalRequestDto.Email,
		Phone:            loginExternalRequestDto.Phone,
		Birthday:         loginExternalRequestDto.Birthday,
	})
}

// LoginGoogleInput is LoginGoogle's request.
type LoginGoogleInput struct {
	JSON api.Body[authModels.LoginGoogleRequestDto]
}

// LoginGoogle is POST /login/google: Sign in with Google (APP-016).
func (uc *AuthController) LoginGoogle(ctx context.Context, in *LoginGoogleInput) (*api.Out[*authModels.LoginSuccessResponse], error) {
	c := api.Gin(ctx)
	if uc.google == nil {
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, authErrors.UserLoginGoogleDisabled))
	}

	var request authModels.LoginGoogleRequestDto
	if err := in.JSON.Into(ctx, &request); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
	}

	identity, err := uc.google.Exchange(c.Request.Context(), request.Code)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.UserLoginGoogleInvalid))
	}

	// An address with an account signs into it, password or not; a new one is created (APP-016 D4).
	email := identity.Email
	return uc.loginExternal(c, authModels.LoginExternalDto{
		AuthProviderName: "google",
		AuthProviderId:   identity.Subject,
		DeviceId:         request.DeviceId,
		FirstName:        optionalString(identity.FirstName),
		LastName:         optionalString(identity.LastName),
		Email:            &email,
	})
}

// loginExternal opens the session of a provider-proved account and issues its tokens.
func (uc *AuthController) loginExternal(c *gin.Context, loginExternal authModels.LoginExternalDto) (*api.Out[*authModels.LoginSuccessResponse], error) {
	declaredClient, err := clientType(c)
	if err != nil {
		return nil, err
	}

	userAgent := c.GetHeader("User-Agent")

	// Parse user agent only if parser is available
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		loginExternal.DeviceInfo = strings.TrimSpace(client.Device.Family)
		loginExternal.DeviceOs = strings.TrimSpace(client.Os.Family)
		loginExternal.Browser = strings.TrimSpace(client.UserAgent.Family)
	}

	loginExternal.IpAddress = utils.GetClientIp(c)
	loginExternal.UserAgent = userAgent
	loginExternal.ClientType = declaredClient

	sessionUser, err := uc.authService.LoginExternal(c.Request.Context(), loginExternal)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(&authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshtoken,
	})
}

// optionalString is nil for an empty claim, so the SP keeps its own default.
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// RefreshTokenInput is RefreshToken's request.
type RefreshTokenInput struct {
	JSON api.Body[authModels.RefreshTokenRequestDto]
}

// RefreshToken is POST /refresh_token: Refresh access token.
func (uc *AuthController) RefreshToken(ctx context.Context, in *RefreshTokenInput) (*api.Out[*authModels.RefreshTokenResponse], error) {
	c := api.Gin(ctx)
	var req authModels.RefreshTokenRequestDto

	if err := in.JSON.Into(ctx, &req); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
	}

	// Use RefreshAccessToken which internally validates the refresh token with the correct secret
	// and returns a new access token
	newAccessToken, err := uc.jwtService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	// Now we need to extract claims from refresh token to validate session
	// We'll parse the token manually to get the claims
	authConf := config.ModularAppConfig.Auth
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(authConf.JWTRefreshKey), nil
	})

	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshInvalid))
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
	}

	// Parse user_id and session_id from claims
	userID, err := uuid.Parse(claims["user_id"].(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
	}

	sessionID, err := uuid.Parse(claims["session_id"].(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
	}

	// Validate that the session is still active in database
	err = uc.authService.ValidateSession(userID, sessionID)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	// A new refresh token rides along (MOBILE-011 D4): its lifetime restarts on every refresh, so a
	// session ends after JWT_REFRESH_EXPIRATION_HOURS of disuse, never while it is being used.
	systemRole := coreModels.SystemRoleUser
	if role, ok := claims["system_role"].(string); ok {
		systemRole = role
	}
	newRefreshToken, err := uc.jwtService.GenerateRefreshToken(userID, sessionID, systemRole)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(&authModels.RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	})
}

// RegisterInput is Register's request.
type RegisterInput struct {
	JSON api.Body[authModels.CreateUserDto]
}

// Register is POST /register: Register.
func (uc *AuthController) Register(ctx context.Context, in *RegisterInput) (*api.Out[*authModels.LoginSuccessResponse], error) {
	c := api.Gin(ctx)
	var newUser authModels.CreateUserDto

	if err := in.JSON.Into(ctx, &newUser); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserRegisterValidationFailed, utils.ExtractValidationError(c, err)))
	}

	if _, err := uc.authService.InsertUser(newUser); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// Auto-login: create a session immediately so the client receives tokens
	// and can proceed to onboarding without a separate login step.
	declaredClient, err := clientType(c)
	if err != nil {
		return nil, err
	}

	userAgent := c.GetHeader("User-Agent")
	var deviceInfo, deviceOs, browser string
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family + " " + client.Os.Major)
		browser = strings.TrimSpace(client.UserAgent.Family + " " + client.UserAgent.Major)
	}

	sessionUser, err := uc.authService.LoginUser(c.Request.Context(), authModels.LoginUserDto{
		Email:      newUser.Email,
		Password:   newUser.Password,
		IpAddress:  utils.GetClientIp(c),
		DeviceInfo: deviceInfo,
		DeviceOs:   deviceOs,
		Browser:    browser,
		UserAgent:  userAgent,
		ClientType: declaredClient,
	})
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	refreshToken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(&authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Logout is POST /logout: Logout.
func (uc *AuthController) Logout(ctx context.Context, _ *api.None) (*api.Out[*bool], error) {
	c := api.Gin(ctx)
	userId, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	sessionId, err := uuid.Parse(c.GetString("session_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	logoutSessionDto := authModels.LogoutSessionDto{
		UserId:    userId,
		SessionId: sessionId,
	}

	unregisteredSession, err := uc.authService.LogoutUser(logoutSessionDto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(unregisteredSession)
}

// GetProfile is GET /profile: Get user profile.
func (uc *AuthController) GetProfile(ctx context.Context, _ *api.None) (*api.Out[*coreModels.User], error) {
	c := api.Gin(ctx)
	var getProfileDto authModels.GetProfileDto
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}
	getProfileDto.ID = userID

	user, err := uc.authService.GetProfile(getProfileDto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}
	if role := c.GetString("system_role"); role != "" {
		user.SystemRole = &role
	}

	return api.OK(user)
}

// UpdateProfileInput is UpdateProfile's request.
type UpdateProfileInput struct {
	JSON api.Body[authModels.UpdateProfileDto]
}

// UpdateProfile is PATCH /profile: Update user profile.
func (uc *AuthController) UpdateProfile(ctx context.Context, in *UpdateProfileInput) (*api.Out[*coreModels.User], error) {
	c := api.Gin(ctx)
	var updateUser authModels.UpdateProfileDto
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}
	updateUser.ID = userID

	if err := in.JSON.Into(ctx, &updateUser); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserProfileValidationFailed, utils.ExtractValidationError(c, err)))
	}
	// A photo is one published to the media bucket, or none (MEDIA-001 D5): an outside link would
	// let its host track whoever views the profile.
	if picture := updateUser.ProfilePictureUrl; picture != nil && *picture != "" && !coreServices.IsMediaURL(*picture) {
		return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, authErrors.ProfilePictureURLInvalid))
	}

	// The language switch is its own statement, run only when one was sent (APP-009 D2).
	if updateUser.Locale != nil {
		if err := uc.authService.SetLocale(c.Request.Context(), userID, *updateUser.Locale); err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
		}
	}

	user, err := uc.authService.UpdateProfile(updateUser)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(user)
}

// GenerateEmailVerificationTokenInput is GenerateEmailVerificationToken's request; the body is
// optional, its email defaulting to the account's own.
type GenerateEmailVerificationTokenInput struct {
	JSON api.Body[authModels.VerifyEmailRequestDto]
}

// GenerateEmailVerificationToken sends an email verification link.
func (uc *AuthController) GenerateEmailVerificationToken(ctx context.Context, in *GenerateEmailVerificationTokenInput) (*api.Out[bool], error) {
	c := api.Gin(ctx)
	var verifyEmailRequestDto authModels.VerifyEmailRequestDto

	if c.Request.ContentLength > 0 {
		if err := in.JSON.Into(ctx, &verifyEmailRequestDto); err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserRequestEmailError, utils.ExtractValidationError(c, err)))
		}
	}

	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	verifyEmailRequest := &authModels.VerifyEmailRequest{
		Email:  verifyEmailRequestDto.Email,
		UserId: &userID,
	}

	// No address in the body verifies the account's own: the email goes to the one it already has.
	recipient := verifyEmailRequestDto.Email
	if recipient == nil {
		user, err := uc.authService.GetProfile(authModels.GetProfileDto{ID: userID})
		if err != nil || user.Email == nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, authErrors.UserRequestEmailError))
		}
		recipient = user.Email
	}

	tx, err := uc.dbService.BeginTransaction(c)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	token, err := uc.authService.GenerateEmailVerificationToken(*verifyEmailRequest, tx)
	if err != nil {
		_ = tx.Rollback(c)
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// Send verification email
	coreConf := config.ModularAppConfig.Core
	lang := c.GetString("lang")
	emailData := map[string]string{
		"VerificationURL": coreConf.FrontendURL + "/confirm-email?token=" + token.Token.String(),
		"Title":           utils.GetTranslation(lang, "verify-email.email.title"),
		"Greeting":        utils.GetTranslation(lang, "verify-email.email.greeting"),
		"Description":     utils.GetTranslation(lang, "verify-email.email.description"),
		"ButtonText":      utils.GetTranslation(lang, "verify-email.email.button-text"),
		"IgnoreText":      utils.GetTranslation(lang, "verify-email.email.ignore-text"),
		"SignOff":         utils.GetTranslation(lang, "verify-email.email.sign-off"),
		"AutomatedNote":   utils.GetTranslation(lang, "verify-email.email.automated-note"),
	}
	subject := utils.GetTranslation(lang, "verify-email.email.subject")

	templatePath := coreServices.GetTemplatePath("auth", "verify-email.html")
	err = uc.emailService.SendEmail(*recipient, subject, templatePath, emailData)
	if err != nil {
		// Not sent → no token (AUTH-002): the user is told and can retry at once. The cause stays in the
		// log; SMTP details are not the client's business.
		_ = tx.Rollback(c)
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, authErrors.UserChangeEmailSendingError))
	}

	err = tx.Commit(c)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// `true`: the token goes out by email only — in the answer it would confirm an address unread.
	return api.OK(true)
}

// ConfirmEmailAddressInput is ConfirmEmailAddress's request.
type ConfirmEmailAddressInput struct {
	JSON api.Body[authModels.VerifyEmailToken]
}

// ConfirmEmailAddress is PUT /email/verification: Confirm email address.
func (uc *AuthController) ConfirmEmailAddress(ctx context.Context, in *ConfirmEmailAddressInput) (*api.Out[*bool], error) {
	c := api.Gin(ctx)
	var verifyEmailRequest authModels.VerifyEmailToken

	if err := in.JSON.Into(ctx, &verifyEmailRequest); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserEmailVerification, utils.ExtractValidationError(c, err)))
	}

	confirmed, err := uc.authService.ConfirmEmailAddress(verifyEmailRequest)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(confirmed)
}

// ChangePasswordInput is ChangePassword's request.
type ChangePasswordInput struct {
	JSON api.Body[authModels.ChangePasswordRequestDto]
}

// ChangePassword is PUT /password: Change user password.
func (uc *AuthController) ChangePassword(ctx context.Context, in *ChangePasswordInput) (*api.Out[*bool], error) {
	c := api.Gin(ctx)
	var changePasswordEequestDto authModels.ChangePasswordRequestDto

	if err := in.JSON.Into(ctx, &changePasswordEequestDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserChangePasswordError, utils.ExtractValidationError(c, err)))
	}

	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	changePasswordDto := authModels.ChangePasswordDto{
		UserId:          &userID,
		PasswordCurrent: changePasswordEequestDto.PasswordCurrent,
		PasswordNew:     changePasswordEequestDto.PasswordNew,
	}
	// The session making the change stays signed in; every other one ends in the same SP call.
	if sessionID, err := uuid.Parse(c.GetString("session_id")); err == nil {
		changePasswordDto.CurrentSessionId = &sessionID
	}

	changed, err := uc.authService.ChangePassword(changePasswordDto)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildError(c, err))
	}

	return api.OK(changed)
}

// GeneratePasswordResetTokenInput is GeneratePasswordResetToken's request.
type GeneratePasswordResetTokenInput struct {
	JSON api.Body[authModels.PasswordResetRequestDto]
}

// GeneratePasswordResetToken is POST /password/reset: Request password reset.
func (uc *AuthController) GeneratePasswordResetToken(ctx context.Context, in *GeneratePasswordResetTokenInput) (*api.Out[bool], error) {
	c := api.Gin(ctx)
	var passwordResetRequestDto authModels.PasswordResetRequestDto

	if err := in.JSON.Into(ctx, &passwordResetRequestDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserPasswordResetError, utils.ExtractValidationError(c, err)))
	}

	tx, err := uc.dbService.BeginTransaction(c)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	token, err := uc.authService.GeneratePasswordResetToken(passwordResetRequestDto, tx)
	if err != nil {
		_ = tx.Rollback(c)
		// An unknown email gets the answer a known one does, and no email.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == authErrors.UserPasswordResetAccountMissing {
			return api.OK(true)
		}
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	// Send password reset email
	coreConf := config.ModularAppConfig.Core
	lang := c.GetString("lang")
	emailData := map[string]string{
		"PasswordResetURL": coreConf.FrontendURL + "/reset-password?token=" + token.Token.String(),
		"Title":            utils.GetTranslation(lang, "password-reset.email.title"),
		"Greeting":         utils.GetTranslation(lang, "password-reset.email.greeting"),
		"Description":      utils.GetTranslation(lang, "password-reset.email.description"),
		"ButtonText":       utils.GetTranslation(lang, "password-reset.email.button-text"),
		"IgnoreText":       utils.GetTranslation(lang, "password-reset.email.ignore-text"),
		"SignOff":          utils.GetTranslation(lang, "password-reset.email.sign-off"),
		"AutomatedNote":    utils.GetTranslation(lang, "password-reset.email.automated-note"),
	}
	subject := utils.GetTranslation(lang, "password-reset.email.subject")

	templatePath := coreServices.GetTemplatePath("auth", "password-reset.html")
	err = uc.emailService.SendEmail(*passwordResetRequestDto.Email, subject, templatePath, emailData)
	if err != nil {
		// Not sent → no token (AUTH-002), so the next try is not refused as "already sent".
		_ = tx.Rollback(c)
		return nil, api.Fail(http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, authErrors.UserForgorPasswordEmailSendingError))
	}

	err = tx.Commit(c)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(true)
}

// ValidateResetTokenInput is ValidateResetToken's request.
type ValidateResetTokenInput struct {
	Query api.Query[authModels.ValidateResetTokenDto]
}

// ValidateResetToken is GET /password/reset: Validate password reset token.
func (uc *AuthController) ValidateResetToken(ctx context.Context, in *ValidateResetTokenInput) (*api.Out[*bool], error) {
	c := api.Gin(ctx)
	var dto authModels.ValidateResetTokenDto

	if err := in.Query.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserPasswordResetError, utils.ExtractValidationError(c, err)))
	}

	valid, err := uc.authService.ValidateResetToken(dto)
	if err != nil {
		return nil, api.Fail(http.StatusGone, coreErrors.BuildErrorSingle(c, authErrors.UserPasswordResetTokenInvalid))
	}

	return api.OK(valid)
}

// ResetPasswordWithTokenInput is ResetPasswordWithToken's request.
type ResetPasswordWithTokenInput struct {
	JSON api.Body[authModels.PasswordResetWithTokenDto]
}

// ResetPasswordWithToken is PUT /password/reset: Reset password with token.
func (uc *AuthController) ResetPasswordWithToken(ctx context.Context, in *ResetPasswordWithTokenInput) (*api.Out[*bool], error) {
	c := api.Gin(ctx)
	var passwordResetWithTokenDto authModels.PasswordResetWithTokenDto

	if err := in.JSON.Into(ctx, &passwordResetWithTokenDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserPasswordResetError, utils.ExtractValidationError(c, err)))
	}

	changed, err := uc.authService.ResetPasswordWithToken(passwordResetWithTokenDto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(changed)
}
