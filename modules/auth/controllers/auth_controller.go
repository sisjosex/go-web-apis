package controllers

import (
	"josex/web/config"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/errors"
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/login [post]
// @Security ApiKeyAuth
func (uc *AuthController) Login(c *gin.Context) {
	var loginUserRequest authModels.LoginUserRequestDto

	if err := c.ShouldBindJSON(&loginUserRequest); err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

	loginUser := authModels.LoginUserDto{
		Email:    loginUserRequest.Email,
		Password: loginUserRequest.Password,
		DeviceId: loginUserRequest.DeviceId,
	}

	loginUser.IpAddress = utils.GetClientIp(c)
	loginUser.DeviceInfo = strings.TrimSpace(client.Device.Family)
	loginUser.DeviceOs = strings.TrimSpace(client.Os.Family + " " + client.Os.Major)
	loginUser.Browser = strings.TrimSpace(client.UserAgent.Family + " " + client.UserAgent.Major)
	loginUser.UserAgent = userAgent

	sessionUser, err := uc.authService.LoginUser(loginUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/login/facebook [post]
// @Security ApiKeyAuth
func (uc *AuthController) LoginFacebook(c *gin.Context) {
	var loginExternalRequestDto authModels.LoginExternalRequestDto

	if err := c.ShouldBindJSON(&loginExternalRequestDto); err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

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
	loginExternal.DeviceInfo = strings.TrimSpace(client.Device.Family)
	loginExternal.DeviceOs = strings.TrimSpace(client.Os.Family)
	loginExternal.Browser = strings.TrimSpace(client.UserAgent.Family)
	loginExternal.UserAgent = userAgent
	// Facabeook Login
	loginExternal.AuthProviderName = "facebook"

	sessionUser, err := uc.authService.LoginExternal(loginExternal)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Router /auth/token/refresh [post]
// @Security ApiKeyAuth
func (uc *AuthController) RefreshToken(c *gin.Context) {
	var req authModels.RefreshTokenRequestDto

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	// Use RefreshAccessToken which internally validates the refresh token with the correct secret
	// and returns a new access token
	newAccessToken, err := uc.jwtService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errors.BuildError(err))
		return
	}

	// Now we need to extract claims from refresh token to validate session
	// We'll parse the token manually to get the claims
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JwtRefreshKey), nil
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, errors.BuildErrorSingle(errors.TokenRefreshInvalid))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, errors.BuildErrorSingle(errors.TokenRefreshClaimsInvalid))
		return
	}

	// Parse user_id and session_id from claims
	userID, err := uuid.Parse(claims["user_id"].(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, errors.BuildErrorSingle(errors.TokenRefreshClaimsInvalid))
		return
	}

	sessionID, err := uuid.Parse(claims["session_id"].(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, errors.BuildErrorSingle(errors.TokenRefreshClaimsInvalid))
		return
	}

	// Validate that the session is still active in database
	err = uc.authService.ValidateSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/register [post]
// @Security ApiKeyAuth
func (uc *AuthController) Register(ctx *gin.Context) {
	var newUser authModels.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.authService.InsertUser(newUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// Logout godoc
// @Summary Logout
// @Description Logout
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} bool
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/logout [post]
// @Security ApiKeyAuth
func (uc *AuthController) Logout(c *gin.Context) {
	userId, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	sessionId, err := uuid.Parse(c.GetString("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	logoutSessionDto := authModels.LogoutSessionDto{
		UserId:    userId,
		SessionId: sessionId,
	}

	unregisteredSession, err := uc.authService.LogoutUser(logoutSessionDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/profile [get]
// @Security ApiKeyAuth
func (uc *AuthController) GetProfile(ctx *gin.Context) {
	var getProfileDto authModels.GetProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}
	getProfileDto.ID = userID

	user, err := uc.authService.GetProfile(getProfileDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/profile [patch]
// @Security ApiKeyAuth
func (uc *AuthController) UpdateProfile(ctx *gin.Context) {
	var updateUser authModels.UpdateProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}
	updateUser.ID = userID

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.authService.UpdateProfile(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/email/verification [post]
// @Security ApiKeyAuth
func (uc *AuthController) GenerateEmailVerificationToken(ctx *gin.Context) {
	var verifyEmailRequestDto authModels.VerifyEmailRequestDto

	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&verifyEmailRequestDto); err != nil {
			ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserRequestEmailError, utils.ExtractValidationError(err)))
			return
		}
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	verifyEmailRequest := &authModels.VerifyEmailRequest{
		Email:  verifyEmailRequestDto.Email,
		UserId: &userID,
	}

	tx, err := uc.dbService.BeginTransaction(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	token, err := uc.authService.GenerateEmailVerificationToken(*verifyEmailRequest, tx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		tx.Rollback(ctx)
		return
	}

	// Send verification email
	emailData := map[string]string{
		"VerificationURL": config.AppConfig.FrontendUrl + "/confirm_email?token=" + token.Token.String(),
	}

	templatePath := coreServices.GetTemplatePath("auth", "verify-email.html")
	err = uc.emailService.SendEmail(*verifyEmailRequest.Email, "Verifica tu cuenta", templatePath, emailData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserChangeEmailSendingError, err.Error()))
		tx.Rollback(ctx)
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/email/verification [put]
// @Security ApiKeyAuth
func (uc *AuthController) ConfirmEmailAddress(ctx *gin.Context) {
	var verifyEmailRequest authModels.VerifyEmailToken

	if err := ctx.ShouldBindJSON(&verifyEmailRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserEmailVerification, utils.ExtractValidationError(err)))
		return
	}

	confirmed, err := uc.authService.ConfirmEmailAddress(verifyEmailRequest)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/password [put]
// @Security ApiKeyAuth
func (uc *AuthController) ChangePassword(ctx *gin.Context) {
	var changePasswordEequestDto authModels.ChangePasswordRequestDto

	if err := ctx.ShouldBindJSON(&changePasswordEequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserChangePasswordError, utils.ExtractValidationError(err)))
		return
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	changePasswordDto := authModels.ChangePasswordDto{
		UserId:          &userID,
		PasswordCurrent: changePasswordEequestDto.PasswordCurrent,
		PasswordNew:     changePasswordEequestDto.PasswordNew,
	}

	changed, err := uc.authService.ChangePassword(changePasswordDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/password/reset [post]
// @Security ApiKeyAuth
func (uc *AuthController) GeneratePasswordResetToken(ctx *gin.Context) {
	var passwordResetRequestDto authModels.PasswordResetRequestDto

	if err := ctx.ShouldBindJSON(&passwordResetRequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserPasswordResetError, utils.ExtractValidationError(err)))
		return
	}

	tx, err := uc.dbService.BeginTransaction(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	token, err := uc.authService.GeneratePasswordResetToken(passwordResetRequestDto, tx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		tx.Rollback(ctx)
		return
	}

	// Send password reset email
	emailData := map[string]string{
		"PasswordResetURL": config.AppConfig.FrontendUrl + "/reset_password?token=" + token.Token.String(),
	}

	templatePath := coreServices.GetTemplatePath("auth", "password-reset.html")
	err = uc.emailService.SendEmail(*passwordResetRequestDto.Email, "Restablece tu contraseña", templatePath, emailData)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserForgorPasswordEmailSendingError, err.Error()))
		tx.Rollback(ctx)
		return
	}

	err = tx.Commit(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, token)
}

// ResetPasswordWithToken godoc
// @Summary Reset password with token
// @Description Reset password using reset token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body authModels.PasswordResetWithTokenDto true "Reset data"
// @Success 200 {object} bool
// @Failure 400 {object} errors.ErrorResponse
// @Router /auth/password/reset [put]
// @Security ApiKeyAuth
func (uc *AuthController) ResetPasswordWithToken(ctx *gin.Context) {
	var passwordResetWithTokenDto authModels.PasswordResetWithTokenDto

	if err := ctx.ShouldBindJSON(&passwordResetWithTokenDto); err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildErrorDetail(errors.UserPasswordResetError, utils.ExtractValidationError(err)))
		return
	}

	changed, err := uc.authService.ResetPasswordWithToken(passwordResetWithTokenDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errors.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}
