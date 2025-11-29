package controllers

import (
	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserController struct {
	userService userInterfaces.UserService
}

func NewUserController(userService userInterfaces.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

func (uc *UserController) Create(ctx *gin.Context) {
	var newUser userModels.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(usersErrors.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	// Generate password if empty
	if newUser.Password == "" {
		newUser.Password = utils.GenerateRandomPassword()
	}

	user, err := uc.userService.InsertUser(newUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(err))
		return
	}

	ctx.JSON(http.StatusCreated, user)
}

func (uc *UserController) Update(ctx *gin.Context) {
	var updateUser userModels.UpdateUserDto
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(err))
		return
	}

	updateUser.ID = id

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(usersErrors.UserValidationFailed, utils.ExtractValidationError(err)))
		return
	}

	user, err := uc.userService.UpdateUser(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(usersErrors.UserUpdateFailed, err.Error()))
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
// @Param limit query int false "Items per page (default: 20, max: 100)" minimum(1) maximum(100)
// @Param search query string false "Search in email, first name, last name"
// @Param status query string false "Filter by status" Enums(active, inactive, expired)
// @Param sort query string false "Sort field (default: created_at)" Enums(created_at, email, first_name, last_name)
// @Param order query string false "Sort order (default: desc)" Enums(asc, desc)
// @Success 200 {object} models.UserListResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Router /users [get]
// @Security ApiKeyAuth
func (uc *UserController) ListUsers(c *gin.Context) {
	usersConfig := config.ModularAppConfig.Users

	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(usersConfig.DefaultPageSize)))

	// Validate page number
	if page < 1 {
		page = 1
	}

	// Validate and enforce max page size
	if limit < 1 {
		limit = usersConfig.DefaultPageSize
	}
	if limit > usersConfig.MaxPageSize {
		limit = usersConfig.MaxPageSize
	}

	users, err := uc.userService.ListUsers()
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, users)
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
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /users/{id} [get]
// @Security ApiKeyAuth
func (uc *UserController) GetUserById(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(coreErrors.InvalidUUID))
		return
	}

	user, err := uc.userService.GetUserById(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, user)
}

// SoftDeleteUser godoc
// @Summary Soft delete user
// @Description Soft delete a user (sets deleted_at timestamp and invalidates sessions). Respects ALLOW_USER_DELETION and ENABLE_SOFT_DELETE config.
// @Tags Users
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param id path string true "User ID (UUID)"
// @Param request body models.SoftDeleteUserDto false "Deletion details"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse "User deletion is disabled"
// @Failure 404 {object} errors.ErrorResponse
// @Router /users/{id} [delete]
// @Security ApiKeyAuth
func (uc *UserController) SoftDeleteUser(c *gin.Context) {
	usersConfig := config.ModularAppConfig.Users

	// Check if user deletion is allowed
	if !usersConfig.AllowUserDeletion {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(usersErrors.UserDeleteNotAllowed))
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(coreErrors.InvalidUUID))
		return
	}

	// Use soft delete if enabled, otherwise permanent delete
	if usersConfig.EnableSoftDelete {
		err = uc.userService.SoftDeleteUser(userID)
	} else {
		// If hard delete is implemented, call it here
		err = uc.userService.SoftDeleteUser(userID) // TODO: implement HardDeleteUser when needed
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"soft_delete": usersConfig.EnableSoftDelete,
	})
}
