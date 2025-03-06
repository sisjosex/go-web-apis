package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserController struct {
	userService interfaces.UserService
}

func NewUserController(userService interfaces.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
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
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	ctx.JSON(http.StatusCreated, user)
}

func (uc *UserController) Update(ctx *gin.Context) {
	var updateUser models.UpdateUserDto
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	updateUser.ID = id

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
