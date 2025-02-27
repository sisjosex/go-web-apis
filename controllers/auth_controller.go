package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/services"
	"josex/web/utils"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

type AuthController struct {
	userService interfaces.UserService
	jwtService  services.JWTService
}

func NewAuthController(userService interfaces.UserService, jwtService services.JWTService) *AuthController {
	return &AuthController{
		userService: userService,
		jwtService:  jwtService,
	}
}

func (uc *AuthController) Login(c *gin.Context) {
	var loginUser models.LoginUserDto

	if err := c.ShouldBindJSON(&loginUser); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserLoginValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	userAgent := c.GetHeader("User-Agent")

	client := parser.Parse(userAgent)

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

	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Fatal(err)
	}

	userAgent := c.GetHeader("User-Agent")

	client := parser.Parse(userAgent)

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
	userId := c.GetString("user_id")
	sessionId := c.GetString("session_id")

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

func (uc *AuthController) UpdateProfile(ctx *gin.Context) {
	var updateUser models.UpdateProfileDto
	updateUser.ID = ctx.GetString("user_id")

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.userService.UpdateProfile(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserUpdateFailed, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, user)
}
