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

func (uc *AuthController) Login(c *gin.Context) {
	var loginUser models.LoginUserDto

	if err := c.ShouldBindJSON(&loginUser); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

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

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshtoken,
	})
}

func (uc *AuthController) LoginFacebook(c *gin.Context) {
	var loginExternal models.LoginExternalDto

	if err := c.ShouldBindJSON(&loginExternal); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")

	client := uc.parser.Parse(userAgent)

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

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshtoken,
	})
}

func (uc *AuthController) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	newAccessToken, err := uc.jwtService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"access_token": newAccessToken})
}

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

	sessionUser := &models.SessionUser{
		UserId:    userId,
		SessionId: sessionId,
	}
	unregisteredSession, err := uc.userService.LogoutUser(*sessionUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, unregisteredSession)
}

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

func (uc *AuthController) GenerateEmailVerificationToken(ctx *gin.Context) {
	var verifyEmailRequest models.VerifyEmailRequest

	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&verifyEmailRequest); err != nil {
			ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserRequestEmailError, utils.ExtractValidationError(err)))
			return
		}
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	verifyEmailRequest.UserId = &userID

	token, err := uc.userService.GenerateEmailVerificationToken(verifyEmailRequest)
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

func (uc *AuthController) ChangePassword(ctx *gin.Context) {
	var changePasswordDto models.ChangePasswordDto

	if err := ctx.ShouldBindJSON(&changePasswordDto); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserChangePasswordError, utils.ExtractValidationError(err)))
		return
	}

	userID, err := uuid.Parse(ctx.GetString("user_id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	changePasswordDto.UserId = &userID

	changed, err := uc.userService.ChangePassword(changePasswordDto)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusOK, changed)
}

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
