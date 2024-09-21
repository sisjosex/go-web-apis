package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService interfaces.UserService
}

func NewUserController(userService interfaces.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

/*func Index(c *gin.Context) {
	searchTerm := c.DefaultQuery("search", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userRepository := repositories.NewUserRepository(services.DB)
	userService := services.NewUserService(services.DB, userRepository)

	users, err := userService.FindUsers(searchTerm, page, pageSize)

	if err != nil {
		c.JSON(http.StatusInternalServerError, common.BuildErrorSingle(err.Error))
		return
	}

	c.JSON(http.StatusOK, users)
}*/

func (uc *UserController) Create(ctx *gin.Context) {
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

	ctx.JSON(http.StatusCreated, user)
}

func (uc *UserController) Update(c *gin.Context) {

}

func (uc *UserController) Show(c *gin.Context) {

}

func (uc *UserController) Delete(c *gin.Context) {

}
