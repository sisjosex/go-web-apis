package controllers

import (
	"josex/web/common"
	"josex/web/services"
	"josex/web/utils"
	"josex/web/validators/user"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func Index(c *gin.Context) {
	searchTerm := c.DefaultQuery("search", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userService := services.NewUserService(services.DB)

	users, err := userService.FindUsers(searchTerm, page, pageSize)

	if err != nil {
		c.JSON(http.StatusInternalServerError, common.BuildErrorSingle(err.Error))
		return
	}

	c.JSON(http.StatusOK, users)
}

func Create(c *gin.Context) {
	var newUser user.CreateUserDto
	var userService = services.NewUserService(services.DB)

	if err := c.ShouldBind(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.FormatValidationErrors(err)))
		return
	}

	userID, err := userService.CreateUser(&newUser)

	if err != nil {
		c.JSON(http.StatusInternalServerError, common.BuildErrorDetail(common.UserCreateFailed, err.Error))
		return
	}

	user, _ := userService.GetUserByID(userID)

	c.JSON(http.StatusCreated, user)
}

func Update(c *gin.Context) {

}

func Show(c *gin.Context) {

}

func Delete(c *gin.Context) {

}
