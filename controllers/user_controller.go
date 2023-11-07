package controllers

import (
	"fmt"
	"josex/web/config"
	"josex/web/services"
	"josex/web/utils"
	"josex/web/validators/user"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func Index(c *gin.Context) {
	searchTerm := c.DefaultQuery("search", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userService := services.NewUserService(services.DB)

	users, err := userService.FindUsers(searchTerm, page, pageSize)

	if err != nil {
		c.JSON(http.StatusInternalServerError, config.BuildErrorSingle(err.Error))
		return
	}

	c.JSON(http.StatusOK, users)
}

func Create(c *gin.Context) {
	var email = c.PostForm("email")
	var newUser user.CreateUserDto
	var userService = services.NewUserService(services.DB)

	fmt.Println("email", email)

	binding.Validator.Engine().(*validator.Validate).RegisterValidation("emailexist", func(fl validator.FieldLevel) bool {
		user, _ := userService.GetUserByEmail(email, nil)
		return email != "" && user == nil
	})

	if err := c.ShouldBind(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, config.BuildErrorDetail(config.UserValidationFailed, utils.FormatValidationErrors(err)))
		return
	}

	userID, err := userService.CreateUser(&newUser)

	if err != nil {
		c.JSON(http.StatusInternalServerError, config.BuildErrorDetail(config.UserCreateFailed, err.Error))
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
