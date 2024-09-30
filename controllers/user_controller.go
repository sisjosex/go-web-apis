package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

type UserController struct {
	userService interfaces.UserService
	parser      *uaparser.Parser
}

func NewUserController(userService interfaces.UserService, parser *uaparser.Parser) *UserController {
	return &UserController{
		userService: userService,
		parser:      parser,
	}
}

func (uc *UserController) Register(ctx *gin.Context) {
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

func (uc *UserController) Create(ctx *gin.Context) {
	var newUser models.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	// Generate password if empty
	if newUser.Password == "" {
		newUser.Password = utils.GenerateRandomPassword()
	}

	user, err := uc.userService.InsertUser(newUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserCreateFailed, err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, user)
}

func (uc *UserController) Update(ctx *gin.Context) {
	var updateUser models.UpdateUserDto
	updateUser.ID = ctx.Param("id")

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.userService.UpdateUser(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserUpdateFailed, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, user)
}

func (uc *UserController) Login(c *gin.Context) {
	var loginUser models.LoginUserDto

	if err := c.ShouldBindJSON(&loginUser); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	userAgentParsed := uc.parser.Parse(userAgent)

	// Add the IP address, device info, device OS, browser, and user agent to the loginUser struct
	loginUser.IpAddress = c.ClientIP()
	loginUser.DeviceInfo = userAgentParsed.Device.Family
	loginUser.DeviceOs = userAgentParsed.Os.Family
	loginUser.Browser = userAgentParsed.UserAgent.Family
	loginUser.UserAgent = userAgent

	token, err := uc.userService.LoginUser(loginUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserCreateFailed, err.Error()))
		return
	}

	c.JSON(http.StatusOK, token)
}

func (uc *UserController) Delete(c *gin.Context) {

}
