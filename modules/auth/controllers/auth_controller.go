package controllers

import (
	"josex/web/config"
	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models" // Used in Swagger annotations
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
}

func NewAuthController(
	authService authInterfaces.AuthService,
	jwtService services.JWTService,
	emailService coreServices.EmailService,
	agentParser *uaparser.Parser,
	db coreServices.DatabaseService,
) *AuthController {
	return &AuthController{
		authService:  authService,
		jwtService:   jwtService,
		emailService: emailService,
		parser:       agentParser,
		dbService:    db,
	}
}

// Login godoc
// @Summary Allow login based on email authentication, email can be confirmed after login
// @Description Allow login based on email authentication, email can be confirmed after login
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.LoginUserRequestDto true "Datos de usuario"
// @Success 200 {object} authModels.LoginSuccessResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/login [post]
// @Security ApiKeyAuth
func (uc *AuthController) Login(c *gin.Context) {
	var loginUserRequest authModels.LoginUserRequestDto

	if err := c.ShouldBindJSON(&loginUserRequest); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
		return
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

	sessionUser, err := uc.authService.LoginUser(loginUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, &authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshtoken,
	})
}

// LoginFacebook godoc
// @Summary Allow login based on Facebook authentication
// @Description Allow login based on Facebook authentication
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.LoginExternalRequestDto true "Datos de usuario"
// @Success 200 {object} authModels.LoginSuccessResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/login/facebook [post]
// @Security ApiKeyAuth
func (uc *AuthController) LoginFacebook(c *gin.Context) {
	var loginExternalRequestDto authModels.LoginExternalRequestDto

	if err := c.ShouldBindJSON(&loginExternalRequestDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	var deviceInfo, deviceOs, browser string

	// Parse user agent only if parser is available
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family)
		browser = strings.TrimSpace(client.UserAgent.Family)
	}

	loginExternal := authModels.LoginExternalDto{
		AuthProviderId: loginExternalRequestDto.AuthProviderId,
		DeviceId:       loginExternalRequestDto.DeviceId,
		FirstName:      loginExternalRequestDto.FirstName,
		LastName:       loginExternalRequestDto.LastName,
		Email:          loginExternalRequestDto.Email,
		Phone:          loginExternalRequestDto.Phone,
		Birthday:       loginExternalRequestDto.Birthday,
	}

	loginExternal.IpAddress = utils.GetClientIp(c)
	loginExternal.DeviceInfo = deviceInfo
	loginExternal.DeviceOs = deviceOs
	loginExternal.Browser = browser
	loginExternal.UserAgent = userAgent
	// Facabeook Login
	loginExternal.AuthProviderName = "facebook"

	sessionUser, err := uc.authService.LoginExternal(loginExternal)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, &authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshtoken,
	})
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token and validate active session
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.RefreshTokenRequestDto true "Refresh token"
// @Success 200 {object} authModels.RefreshTokenResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /auth/token/refresh [post]
// @Security ApiKeyAuth
func (uc *AuthController) RefreshToken(c *gin.Context) {
	var req authModels.RefreshTokenRequestDto

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.UserLoginValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	// Use RefreshAccessToken which internally validates the refresh token with the correct secret
	// and returns a new access token
	newAccessToken, err := uc.jwtService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	// Now we need to extract claims from refresh token to validate session
	// We'll parse the token manually to get the claims
	authConf := config.ModularAppConfig.Auth
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(authConf.JWTRefreshKey), nil
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshInvalid))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
		return
	}

	// Parse user_id and session_id from claims
	userID, err := uuid.Parse(claims["user_id"].(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
		return
	}

	sessionID, err := uuid.Parse(claims["session_id"].(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, authErrors.TokenRefreshClaimsInvalid))
		return
	}

	// Validate that the session is still active in database
	err = uc.authService.ValidateSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, &authModels.RefreshTokenResponse{
		AccessToken: newAccessToken,
	})
}

// Register godoc
// @Summary Register
// @Description Register
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.CreateUserDto true "User"
// @Success 200 {object} coreModels.User
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/register [post]
// @Security ApiKeyAuth
func (uc *AuthController) Register(ctx *gin.Context) {
	var newUser authModels.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserRegisterValidationFailed, utils.ExtractValidationError(ctx, err)))
		return
	}

	if _, err := uc.authService.InsertUser(newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	// Auto-login: create a session immediately so the client receives tokens
	// and can proceed to onboarding without a separate login step.
	userAgent := ctx.GetHeader("User-Agent")
	var deviceInfo, deviceOs, browser string
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family + " " + client.Os.Major)
		browser = strings.TrimSpace(client.UserAgent.Family + " " + client.UserAgent.Major)
	}

	sessionUser, err := uc.authService.LoginUser(authModels.LoginUserDto{
		Email:      newUser.Email,
		Password:   newUser.Password,
		IpAddress:  utils.GetClientIp(ctx),
		DeviceInfo: deviceInfo,
		DeviceOs:   deviceOs,
		Browser:    browser,
		UserAgent:  userAgent,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, coreErrors.BuildError(ctx, err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, coreErrors.BuildError(ctx, err))
		return
	}

	refreshToken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId, sessionUser.SystemRole)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, &authModels.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Logout godoc
// @Summary Logout
// @Description Logout
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/logout [post]
// @Security ApiKeyAuth
func (uc *AuthController) Logout(c *gin.Context) {
	userId, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	sessionId, err := uuid.Parse(c.GetString("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	logoutSessionDto := authModels.LogoutSessionDto{
		UserId:    userId,
		SessionId: sessionId,
	}

	unregisteredSession, err := uc.authService.LogoutUser(logoutSessionDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, unregisteredSession)
}

// GetProfile godoc
// @Summary Get user profile
// @Description Get authenticated user profile information
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} coreModels.User
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/profile [get]
// @Security ApiKeyAuth
func (uc *AuthController) GetProfile(ctx *gin.Context) {
	var getProfileDto authModels.GetProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}
	getProfileDto.ID = userID

	user, err := uc.authService.GetProfile(getProfileDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update authenticated user profile information (partial updates allowed)
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body authModels.UpdateProfileDto true "Profile data"
// @Success 200 {object} coreModels.User
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/profile [patch]
// @Security ApiKeyAuth
func (uc *AuthController) UpdateProfile(ctx *gin.Context) {
	var updateUser authModels.UpdateProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}
	updateUser.ID = userID

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserProfileValidationFailed, utils.ExtractValidationError(ctx, err)))
		return
	}

	user, err := uc.authService.UpdateProfile(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// GenerateEmailVerificationToken godoc
// @Summary Request email verification
// @Description Generate and send email verification token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body authModels.VerifyEmailRequestDto false "Email data"
// @Success 200 {object} authModels.VerifyEmailToken
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/email/verification [post]
// @Security ApiKeyAuth
func (uc *AuthController) GenerateEmailVerificationToken(ctx *gin.Context) {
	var verifyEmailRequestDto authModels.VerifyEmailRequestDto

	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&verifyEmailRequestDto); err != nil {
			ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserRequestEmailError, utils.ExtractValidationError(ctx, err)))
			return
		}
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	verifyEmailRequest := &authModels.VerifyEmailRequest{
		Email:  verifyEmailRequestDto.Email,
		UserId: &userID,
	}

	tx, err := uc.dbService.BeginTransaction(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	token, err := uc.authService.GenerateEmailVerificationToken(*verifyEmailRequest, tx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		tx.Rollback(ctx)
		return
	}

	// Send verification email
	coreConf := config.ModularAppConfig.Core
	lang := ctx.GetString("lang")
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
	err = uc.emailService.SendEmail(*verifyEmailRequest.Email, subject, templatePath, emailData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserChangeEmailSendingError, err.Error()))
		tx.Rollback(ctx)
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, token)
}

// ConfirmEmailAddress godoc
// @Summary Confirm email address
// @Description Confirm email address with verification token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.VerifyEmailToken true "Verification token"
// @Success 200 {object} bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/email/verification [put]
// @Security ApiKeyAuth
func (uc *AuthController) ConfirmEmailAddress(ctx *gin.Context) {
	var verifyEmailRequest authModels.VerifyEmailToken

	if err := ctx.ShouldBindJSON(&verifyEmailRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserEmailVerification, utils.ExtractValidationError(ctx, err)))
		return
	}

	confirmed, err := uc.authService.ConfirmEmailAddress(verifyEmailRequest)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, confirmed)
}

// ChangePassword godoc
// @Summary Change user password
// @Description Change password for authenticated user
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body authModels.ChangePasswordRequestDto true "Password data"
// @Success 200 {object} bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/password [put]
// @Security ApiKeyAuth
func (uc *AuthController) ChangePassword(ctx *gin.Context) {
	var changePasswordEequestDto authModels.ChangePasswordRequestDto

	if err := ctx.ShouldBindJSON(&changePasswordEequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserChangePasswordError, utils.ExtractValidationError(ctx, err)))
		return
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	changePasswordDto := authModels.ChangePasswordDto{
		UserId:          &userID,
		PasswordCurrent: changePasswordEequestDto.PasswordCurrent,
		PasswordNew:     changePasswordEequestDto.PasswordNew,
	}

	changed, err := uc.authService.ChangePassword(changePasswordDto)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}

// GeneratePasswordResetToken godoc
// @Summary Request password reset
// @Description Generate and send password reset token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.PasswordResetRequestDto true "Account data"
// @Success 200 {object} authModels.PasswordResetTokenRequestDto
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/password/reset [post]
// @Security ApiKeyAuth
func (uc *AuthController) GeneratePasswordResetToken(ctx *gin.Context) {
	var passwordResetRequestDto authModels.PasswordResetRequestDto

	if err := ctx.ShouldBindJSON(&passwordResetRequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserPasswordResetError, utils.ExtractValidationError(ctx, err)))
		return
	}

	tx, err := uc.dbService.BeginTransaction(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	token, err := uc.authService.GeneratePasswordResetToken(passwordResetRequestDto, tx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		tx.Rollback(ctx)
		return
	}

	// Send password reset email
	coreConf := config.ModularAppConfig.Core
	lang := ctx.GetString("lang")
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
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserForgorPasswordEmailSendingError, err.Error()))
		tx.Rollback(ctx)
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, token)
}

// ValidateResetToken godoc
// @Summary Validate password reset token
// @Description Check whether a password reset token is still valid (not used, not expired)
// @Tags Auth
// @Produce json
// @Param token query string true "Reset token"
// @Success 200 {object} bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 410 {object} coreErrors.ErrorResponse
// @Router /auth/password/reset [get]
func (uc *AuthController) ValidateResetToken(ctx *gin.Context) {
	var dto authModels.ValidateResetTokenDto

	if err := ctx.ShouldBindQuery(&dto); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserPasswordResetError, utils.ExtractValidationError(ctx, err)))
		return
	}

	valid, err := uc.authService.ValidateResetToken(dto)
	if err != nil {
		ctx.JSON(http.StatusGone, coreErrors.BuildErrorSingle(ctx, authErrors.UserPasswordResetTokenInvalid))
		return
	}

	ctx.JSON(http.StatusOK, valid)
}

// ResetPasswordWithToken godoc
// @Summary Reset password with token
// @Description Reset password using reset token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.PasswordResetWithTokenDto true "Reset data"
// @Success 200 {object} bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /auth/password/reset [put]
// @Security ApiKeyAuth
func (uc *AuthController) ResetPasswordWithToken(ctx *gin.Context) {
	var passwordResetWithTokenDto authModels.PasswordResetWithTokenDto

	if err := ctx.ShouldBindJSON(&passwordResetWithTokenDto); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, authErrors.UserPasswordResetError, utils.ExtractValidationError(ctx, err)))
		return
	}

	changed, err := uc.authService.ResetPasswordWithToken(passwordResetWithTokenDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}
