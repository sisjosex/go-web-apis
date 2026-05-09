package controllers

import (
	"errors"
	"fmt"
	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserController struct {
	userService  userInterfaces.UserService
	emailService coreServices.EmailService
}

func NewUserController(userService userInterfaces.UserService, emailService coreServices.EmailService) *UserController {
	return &UserController{
		userService:  userService,
		emailService: emailService,
	}
}

func (uc *UserController) Create(ctx *gin.Context) {
	var newUser userModels.CreateUserDto

	if err := ctx.ShouldBindJSON(&newUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, usersErrors.UserValidationFailed, utils.ExtractValidationError(ctx, err)))
		return
	}

	// Generate a secure temporary password when the admin left it blank
	tempPassword := newUser.Password
	if tempPassword == "" {
		tempPassword = utils.GenerateRandomPassword()
		newUser.Password = tempPassword
	}

	user, err := uc.userService.InsertUser(newUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == "user.create.email.already-exists" {
			ctx.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(ctx, usersErrors.UserEmailAlreadyInUse))
			return
		}
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	// If operating inside a tenant context, add the new user as a member of that tenant
	if tenantIDStr, ok := ctx.Get("tenant_id"); ok {
		if tenantID, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			if requesterIDStr, ok := ctx.Get("user_id"); ok {
				if requesterID, err := uuid.Parse(requesterIDStr.(string)); err == nil {
					if newUserID, err := uuid.Parse(user.ID); err == nil {
						_ = uc.userService.AssignToTenant(tenantID, requesterID, newUserID, "member")
					}
				}
			}
		}
	}

	// Send invitation email (fire-and-forget — don't fail the request if email fails)
	appConf := config.ModularAppConfig.Core
	go func() {
		templatePath := coreServices.GetTemplatePath("users", "user-invitation.html")
		firstName := ""
		if user.FirstName != nil {
			firstName = *user.FirstName
		}
		email := ""
		if user.Email != nil {
			email = *user.Email
		}
		data := map[string]string{
			"Title":              fmt.Sprintf("Welcome to %s", appConf.AppName),
			"Greeting":           fmt.Sprintf("Welcome, %s!", firstName),
			"Description":        fmt.Sprintf("Your account has been created on %s. Use the credentials below to sign in.", appConf.AppName),
			"Email":              email,
			"TempPassword":       tempPassword,
			"ChangePasswordNote": "For security, please change your password after your first login.",
			"LoginURL":           fmt.Sprintf("%s/login", appConf.FrontendURL),
			"ButtonText":         "Sign in now",
			"SignOff":            fmt.Sprintf("The %s team", appConf.AppName),
			"AutomatedNote":      "This is an automated message. Please do not reply.",
		}
		_ = uc.emailService.SendEmail(email, fmt.Sprintf("Your %s account is ready", appConf.AppName), templatePath, data)
	}()

	ctx.JSON(http.StatusOK, user)
}

func (uc *UserController) Update(ctx *gin.Context) {
	var updateUser userModels.UpdateUserDto
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildError(ctx, err))
		return
	}

	updateUser.ID = id

	if err := ctx.ShouldBindJSON(&updateUser); err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, usersErrors.UserValidationFailed, utils.ExtractValidationError(ctx, err)))
		return
	}

	user, err := uc.userService.UpdateUser(updateUser)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(ctx, usersErrors.UserUpdateFailed, err.Error()))
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

	query := userModels.UserListQuery{
		Page:   page,
		Limit:  limit,
		Search: c.Query("search"),
		Status: c.Query("status"),
		Sort:   c.Query("sort"),
		Order:  c.Query("order"),
	}

	// Scope results to the current tenant (set by tenancy middleware)
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			query.TenantID = tid
		}
	}

	// Exclude the calling user from their own results
	if userIDStr, ok := c.Get("user_id"); ok {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			query.ExcludeUserID = &uid
		}
	}

	response, err := uc.userService.ListUsers(query)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
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
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Router /users/{id} [get]
// @Security ApiKeyAuth
func (uc *UserController) GetUserById(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	user, err := uc.userService.GetUserById(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, coreErrors.BuildError(c, err))
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
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, usersErrors.UserDeleteNotAllowed))
		return
	}

	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
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
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"soft_delete": usersConfig.EnableSoftDelete,
	})
}
