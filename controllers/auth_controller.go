package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/utils"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

type AuthController struct {
	userService interfaces.UserService
}

func NewAuthController(userService interfaces.UserService) *AuthController {
	return &AuthController{
		userService: userService,
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

	token, err := uc.userService.LoginUser(loginUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, token)
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

	token, err := uc.userService.LoginExternal(loginExternal)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, token)
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
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserCreateFailed, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, user)
}
