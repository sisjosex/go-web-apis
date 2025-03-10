package controllers

import (
	"josex/web/common"
	"josex/web/config"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/services"
	"josex/web/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ua-parser/uap-go/uaparser"
)

type AuthController struct {
	userService interfaces.UserService
	jwtService  services.JWTService
	parser      *uaparser.Parser
}

func NewAuthController(userService interfaces.UserService, jwtService services.JWTService, agentParser *uaparser.Parser) *AuthController {
	return &AuthController{
		userService: userService,
		jwtService:  jwtService,
		parser:      agentParser,
	}
}

// Login godoc
// @Summary Allow login based on email authentication, email can be confirmed after login
// @Description Allow login based on email authentication, email can be confirmed after login
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.LoginUserRequestDto true "Datos de usuario"
// @Success 200 {object} models.LoginSuccessResponse
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/login [post]
// @Security ApiKeyAuth
func (uc *AuthController) Login(c *gin.Context) {
	var loginUserRequest models.LoginUserRequestDto

	if err := c.ShouldBindJSON(&loginUserRequest); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

	loginUser := models.LoginUserDto{
		Email:    loginUserRequest.Email,
		Password: loginUserRequest.Password,
		DeviceId: loginUserRequest.DeviceId,
	}

	loginUser.IpAddress = utils.GetClientIp(c)
	loginUser.DeviceInfo = strings.TrimSpace(client.Device.Family)
	loginUser.DeviceOs = strings.TrimSpace(client.Os.Family + " " + client.Os.Major)
	loginUser.Browser = strings.TrimSpace(client.UserAgent.Family + " " + client.UserAgent.Major)
	loginUser.UserAgent = userAgent

	sessionUser, err := uc.userService.LoginUser(loginUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, &models.LoginSuccessResponse{
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
// @Param request body models.LoginExternalRequestDto true "Datos de usuario"
// @Success 200 {object} models.LoginSuccessResponse
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/login/facebook [post]
// @Security ApiKeyAuth
func (uc *AuthController) LoginFacebook(c *gin.Context) {
	var loginExternalRequestDto models.LoginExternalRequestDto

	if err := c.ShouldBindJSON(&loginExternalRequestDto); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

	loginExternal := models.LoginExternalDto{
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

	sessionUser, err := uc.userService.LoginExternal(loginExternal)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	accessToken, err := uc.jwtService.GenerateAccessToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	refreshtoken, err := uc.jwtService.GenerateRefreshToken(sessionUser.UserId, sessionUser.SessionId)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, &models.LoginSuccessResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshtoken,
	})
}

// RefreshToken godoc
// @Summary Refresh token
// @Description Refresh token
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.RefreshTokenRequestDto true "Refresh token"
// @Success 200 {object} models.RefreshTokenResponse
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/refresh_token [post]
// @Security ApiKeyAuth
func (uc *AuthController) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequestDto

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newAccessToken, err := uc.jwtService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, &models.RefreshTokenResponse{
		AccessToken: newAccessToken,
	})
}

// Register godoc
// @Summary Register
// @Description Register
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.CreateUserDto true "User"
// @Success 200 {object} models.User
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/register [post]
// @Security ApiKeyAuth
func (uc *AuthController) Register(ctx *gin.Context) {
	var newUser models.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.userService.InsertUser(newUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
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
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/logout [post]
// @Security ApiKeyAuth
func (uc *AuthController) Logout(c *gin.Context) {
	userId, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	sessionId, err := uuid.Parse(c.GetString("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	logoutSessionDto := models.LogoutSessionDto{
		UserId:    userId,
		SessionId: sessionId,
	}

	unregisteredSession, err := uc.userService.LogoutUser(logoutSessionDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, unregisteredSession)
}

// GetProfile godoc
// @Summary GetProfile
// @Description GetProfile
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} models.User
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/get_profile [post]
// @Security ApiKeyAuth
func (uc *AuthController) GetProfile(ctx *gin.Context) {
	var getProfileDto models.GetProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}
	getProfileDto.ID = userID

	user, err := uc.userService.GetProfile(getProfileDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// UpdateProfile godoc
// @Summary UpdateProfile
// @Description UpdateProfile
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body models.UpdateProfileDto true "User"
// @Success 200 {object} models.UpdateProfileDto
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/update_profile [post]
// @Security ApiKeyAuth
func (uc *AuthController) UpdateProfile(ctx *gin.Context) {
	var updateUser models.UpdateProfileDto
	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}
	updateUser.ID = userID

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.userService.UpdateProfile(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// GenerateEmailVerificationToken godoc
// @Summary GenerateEmailVerificationToken
// @Description GenerateEmailVerificationToken
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body models.VerifyEmailRequestDto false "User"
// @Success 200 {object} models.VerifyEmailToken
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/request_verify_email [post]
// @Security ApiKeyAuth
func (uc *AuthController) GenerateEmailVerificationToken(ctx *gin.Context) {
	var verifyEmailRequestDto models.VerifyEmailRequestDto

	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&verifyEmailRequestDto); err != nil {
			ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserRequestEmailError, utils.ExtractValidationError(err)))
			return
		}
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	verifyEmailRequest := &models.VerifyEmailRequest{
		Email:  verifyEmailRequestDto.Email,
		UserId: &userID,
	}

	token, err := uc.userService.GenerateEmailVerificationToken(*verifyEmailRequest)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	emailData := services.VerificationTokenEmailData{
		VerificationURL: config.AppConfig.FrontendUrl + "/confirm_email?token=" + token.Token.String(),
	}

	err = services.SendEmailVerificationToken(*verifyEmailRequest.Email, "Verifica tu cuenta", emailData)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, token)
}

// ConfirmEmailAddress godoc
// @Summary ConfirmEmailAddress
// @Description ConfirmEmailAddress
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.VerifyEmailToken true "Verify Email Token"
// @Success 200 {object} bool
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/confirm_email [post]
// @Security ApiKeyAuth
func (uc *AuthController) ConfirmEmailAddress(ctx *gin.Context) {
	var verifyEmailRequest models.VerifyEmailToken

	if err := ctx.ShouldBindJSON(&verifyEmailRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserEmailVerification, utils.ExtractValidationError(err)))
		return
	}

	confirmed, err := uc.userService.ConfirmEmailAddress(verifyEmailRequest)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, confirmed)
}

// ChangePassword godoc
// @Summary ChangePassword
// @Description ChangePassword
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param request body models.ChangePasswordRequestDto true "User"
// @Success 200 {object} bool
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/change_password [post]
// @Security ApiKeyAuth
func (uc *AuthController) ChangePassword(ctx *gin.Context) {
	var changePasswordEequestDto models.ChangePasswordRequestDto

	if err := ctx.ShouldBindJSON(&changePasswordEequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserChangePasswordError, utils.ExtractValidationError(err)))
		return
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	changePasswordDto := models.ChangePasswordDto{
		UserId:          &userID,
		PasswordCurrent: changePasswordEequestDto.PasswordCurrent,
		PasswordNew:     changePasswordEequestDto.PasswordNew,
	}

	changed, err := uc.userService.ChangePassword(changePasswordDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}

// GeneratePasswordResetToken godoc
// @Summary GeneratePasswordResetToken
// @Description GeneratePasswordResetToken
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.PasswordResetRequestDto true "User Account"
// @Success 200 {object} models.PasswordResetTokenRequestDto
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/request_password_reset [post]
// @Security ApiKeyAuth
func (uc *AuthController) GeneratePasswordResetToken(ctx *gin.Context) {
	var passwordResetRequestDto models.PasswordResetRequestDto

	if err := ctx.ShouldBindJSON(&passwordResetRequestDto); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserPasswordResetError, utils.ExtractValidationError(err)))
		return
	}

	token, err := uc.userService.GeneratePasswordResetToken(passwordResetRequestDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	emailData := services.PasswordResetEmailData{
		PasswordResetURL: config.AppConfig.FrontendUrl + "/reset_password?token=" + token.Token.String(),
	}

	err = services.SendPasswordResetToken(*passwordResetRequestDto.Email, "Restablece tu contraseña", emailData)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, token)
}

// ResetPasswordWithToken godoc
// @Summary ResetPasswordWithToken
// @Description ResetPasswordWithToken
// @Tags Auth
// @Accept  json
// @Produce  json
// @Param request body models.PasswordResetWithTokenDto false "User"
// @Success 200 {object} bool
// @Failure 400 {object} common.ErrorResponse
// @Router /auth/password_reset [post]
// @Security ApiKeyAuth
func (uc *AuthController) ResetPasswordWithToken(ctx *gin.Context) {
	var passwordResetWithTokenDto models.PasswordResetWithTokenDto

	if err := ctx.ShouldBindJSON(&passwordResetWithTokenDto); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserPasswordResetError, utils.ExtractValidationError(err)))
		return
	}

	changed, err := uc.userService.ResetPasswordWithToken(passwordResetWithTokenDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}
