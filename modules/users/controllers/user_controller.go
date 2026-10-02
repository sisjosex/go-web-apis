package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
)

// Ensure coreModels is used (for Swagger annotations)
var _ coreModels.User

type UserController struct {
	userService      userInterfaces.UserService
	emailService     coreServices.EmailService
	userAuditService userInterfaces.UserAuditService
}

func NewUserController(userService userInterfaces.UserService, emailService coreServices.EmailService, userAuditService userInterfaces.UserAuditService) *UserController {
	return &UserController{
		userService:      userService,
		emailService:     emailService,
		userAuditService: userAuditService,
	}
}

func (uc *UserController) Create(c *gin.Context) {
	var newUser userModels.CreateUserDto

	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserValidationFailed, utils.ExtractValidationError(c, err)))
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
			c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, usersErrors.UserEmailAlreadyInUse))
			return
		}
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	// If operating inside a tenant context, add the new user as a member of that tenant
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tenantID, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			if requesterIDStr, ok := c.Get("user_id"); ok {
				if requesterID, err := uuid.Parse(requesterIDStr.(string)); err == nil {
					if newUserID, err := uuid.Parse(user.ID); err == nil {
						tenantRole := newUser.TenantRole
						if tenantRole == "" {
							tenantRole = tenancyModels.RoleMember
						}
						_ = uc.userService.AssignToTenant(tenantID, requesterID, newUserID, tenantRole)
					}
				}
			}

			var performedByPtr *uuid.UUID
			if userIDStr, ok := c.Get("user_id"); ok {
				if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
					performedByPtr = &uid
				}
			}
			var targetIDPtr *uuid.UUID
			if newUserID, err := uuid.Parse(user.ID); err == nil {
				targetIDPtr = &newUserID
			}
			auditSvc := uc.userAuditService
			go func() {
				_ = auditSvc.Record(tenantID, "user.created", targetIDPtr, performedByPtr, nil, "user")
			}()
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

	c.JSON(http.StatusCreated, user)
}

func (uc *UserController) Update(c *gin.Context) {
	var updateUser userModels.UpdateUserDto
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	updateUser.ID = id

	if err := c.ShouldBindJSON(&updateUser); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	// Pass tenant and performer to the repository so the AFTER UPDATE trigger
	// can record the field-level diff atomically within the same transaction.
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tenantID, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			updateUser.TenantID = tenantID
		}
	}
	if userIDStr, ok := c.Get("user_id"); ok {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			updateUser.PerformedBy = &uid
		}
	}

	user, err := uc.userService.UpdateUser(updateUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserUpdateFailed, err.Error()))
		return
	}

	c.JSON(http.StatusOK, user)
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
// @Success 200 {object} userModels.UserListResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /users [get]
// @Security ApiKeyAuth
func (uc *UserController) GetStats(c *gin.Context) {
	var tenantID uuid.UUID
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			tenantID = tid
		}
	}

	var excludeUserID *uuid.UUID
	if userIDStr, ok := c.Get("user_id"); ok {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			excludeUserID = &uid
		}
	}

	stats, err := uc.userService.GetStats(tenantID, excludeUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, stats)
}

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
// @Success 200 {object} coreModels.User
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
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
// @Param request body userModels.SoftDeleteUserDto false "Deletion details"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse "User deletion is disabled"
// @Failure 404 {object} coreErrors.ErrorResponse
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

	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tenantID, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			var performedByPtr *uuid.UUID
			if userIDStr, ok := c.Get("user_id"); ok {
				if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
					performedByPtr = &uid
				}
			}
			targetID := userID
			auditSvc := uc.userAuditService
			go func() {
				_ = auditSvc.Record(tenantID, "user.deleted", &targetID, performedByPtr, nil, "user")
			}()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"soft_delete": usersConfig.EnableSoftDelete,
	})
}

// ResetPassword godoc
// @Summary      Trigger a password reset email for a user
// @Description  Generates a password reset token and sends the reset email to the target user. Always returns 200 to avoid user enumeration.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer Token"
// @Param        id             path    string  true  "User UUID"
// @Success      200
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /users/{id}/reset-password [post]
// @Security     ApiKeyAuth
func (uc *UserController) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	result, err := uc.userService.ResetPassword(id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "user.not-found",
				"password.reset.account-not-exists",
				"password.reset.token-already-sent":
				c.JSON(http.StatusOK, nil)
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tenantID, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			var performedByPtr *uuid.UUID
			if userIDStr, ok := c.Get("user_id"); ok {
				if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
					performedByPtr = &uid
				}
			}
			targetID := id
			auditSvc := uc.userAuditService
			go func() {
				_ = auditSvc.Record(tenantID, "user.password-reset", &targetID, performedByPtr, nil, "user")
			}()
		}
	}

	appConf := config.ModularAppConfig.Core
	lang := c.GetString("lang")
	emailData := map[string]string{
		"PasswordResetURL": fmt.Sprintf("%s/reset-password?token=%s", appConf.FrontendURL, result.Token),
		"Title":            utils.GetTranslation(lang, "password-reset.email.title"),
		"Greeting":         utils.GetTranslation(lang, "password-reset.email.greeting"),
		"Description":      utils.GetTranslation(lang, "password-reset.email.description"),
		"ButtonText":       utils.GetTranslation(lang, "password-reset.email.button-text"),
		"IgnoreText":       utils.GetTranslation(lang, "password-reset.email.ignore-text"),
		"SignOff":          utils.GetTranslation(lang, "password-reset.email.sign-off"),
		"AutomatedNote":    utils.GetTranslation(lang, "password-reset.email.automated-note"),
	}
	subject := utils.GetTranslation(lang, "password-reset.email.subject")
	templatePath := coreServices.GetTemplatePath("auth", "password-reset.html")

	go func() {
		_ = uc.emailService.SendEmail(result.Email, subject, templatePath, emailData)
	}()

	c.JSON(http.StatusOK, nil)
}
