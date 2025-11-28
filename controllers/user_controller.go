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

// ListUsers godoc
// @Summary List users with pagination and filters
// @Description Get paginated list of users with optional search and filters
// @Tags Users
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param page query int false "Page number (default: 1)" minimum(1)
// @Param limit query int false "Items per page (default: 10, max: 100)" minimum(1) maximum(100)
// @Param search query string false "Search in email, first name, last name"
// @Param status query string false "Filter by status" Enums(active, inactive, expired)
// @Param sort query string false "Sort field (default: created_at)" Enums(created_at, email, first_name, last_name)
// @Param order query string false "Sort order (default: desc)" Enums(asc, desc)
// @Success 200 {object} models.UserListResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /users [get]
// @Security ApiKeyAuth
func (uc *UserController) ListUsers(c *gin.Context) {
	var query models.UserListQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorDetail(common.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	response, err := uc.userService.ListUsers(query)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetUserById godoc
// @Summary Get user by ID
// @Description Get a single user by their ID
// @Tags Users
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} models.User
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /users/{id} [get]
// @Security ApiKeyAuth
func (uc *UserController) GetUserById(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	user, err := uc.userService.GetUserById(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SoftDeleteUser godoc
// @Summary Soft delete user
// @Description Soft delete a user (sets deleted_at timestamp and invalidates sessions)
// @Tags Users
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param id path string true "User ID (UUID)"
// @Param request body models.SoftDeleteUserDto false "Deletion details"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /users/{id} [delete]
// @Security ApiKeyAuth
func (uc *UserController) SoftDeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	var deleteDto models.SoftDeleteUserDto
	// Reason is optional, so don't fail if binding fails
	_ = c.ShouldBindJSON(&deleteDto)

	err = uc.userService.SoftDeleteUser(userID, deleteDto.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
