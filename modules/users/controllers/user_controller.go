package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/config"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/limits"
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/core/registry"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyModels "josex/web/modules/tenancy/models"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
	userServices "josex/web/modules/users/services"
)

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

// requestTenant is the workspace the tenancy middleware resolved; the user routes only reach a user there
// (USERS-013). A request without one resolves to uuid.Nil, which matches no membership.
func requestTenant(c *gin.Context) uuid.UUID {
	tid, _ := contextUUID(c, "tenant_id")
	return tid
}

// contextUUID is the UUID a middleware stored under key, false when it is absent or not a UUID.
func contextUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	s, ok := c.Get(key)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s.(string))
	return id, err == nil
}

// CreateUserInput is POST /users: the new account.
type CreateUserInput struct {
	JSON api.Body[userModels.CreateUserDto]
}

// UpdateUserInput is PUT /users/{id}: the fields to change.
type UpdateUserInput struct {
	ID   string `path:"id"`
	JSON api.Body[userModels.UpdateUserDto]
}

// ListUsersInput is GET /users' query. A page or limit below 1 falls back to the default; a limit
// over the configured maximum is capped.
type ListUsersInput struct {
	Page    int    `query:"page" default:"1" doc:"Page number"`
	Limit   int    `query:"limit" doc:"Items per page; the configured default when absent, capped at the maximum"`
	Search  string `query:"search" doc:"Search in email, first name, last name"`
	Status  string `query:"status" doc:"active, inactive or expired"`
	Sort    string `query:"sort" doc:"created_at (default), email, first_name or last_name"`
	Order   string `query:"order" doc:"asc or desc (default)"`
	Level   string `query:"level" doc:"portal, driver, organization, web or admin; any other value is a 400. With none, the app-only portal and driver accounts stay out"`
	RoleID  string `query:"role_id" doc:"Only the holders of this role"`
	Pending bool   `query:"pending" doc:"Only the people who never signed in"`
}

// SoftDeleteResult is DELETE /users/{id}'s answer.
type SoftDeleteResult struct {
	Success    bool `json:"success"`
	SoftDelete bool `json:"soft_delete"`
}

// pgMessage answers the stored procedure's error key, or "" for any other error.
func pgMessage(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Message
	}
	return ""
}

func (uc *UserController) Create(ctx context.Context, in *CreateUserInput) (*api.Out[*coreModels.User], error) {
	c := api.Gin(ctx)
	var newUser userModels.CreateUserDto

	if err := in.JSON.Into(ctx, &newUser); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserValidationFailed, utils.ExtractValidationError(c, err)))
	}

	// Invited with only an email (USERS-015): they sign in with a code, so nobody sees this password.
	invitedByCode := newUser.Password == ""
	tempPassword := newUser.Password
	if invitedByCode {
		tempPassword = utils.GenerateRandomPassword()
		newUser.Password = tempPassword
	}
	if newUser.FirstName == "" {
		newUser.FirstName, _, _ = strings.Cut(newUser.Email, "@")
	}

	// Inside a workspace, a seat must be free before the account exists (BILLING-001 D2).
	tenantID, inTenant := contextUUID(c, "tenant_id")
	if inTenant {
		if err := limits.Refuse(c, uc.userService.EnsureTenantSeat(c.Request.Context(), tenantID)); err != nil {
			return nil, err
		}
		newUser.Locale, _ = registry.TenantSetting(c.Request.Context(), c.GetString("tenant_slug"), tenancyModels.SettingLocale).(string)
	}

	user, err := uc.userService.InsertUser(newUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == "user.create.email.already-exists" {
			return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, usersErrors.UserEmailAlreadyInUse))
		}
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	if inTenant {
		uc.joinTenant(c, tenantID, user, newUser.TenantRole)
		if err := uc.assignRoles(c, tenantID, user, newUser.RoleIDs); err != nil {
			return nil, err
		}
	}
	if invitedByCode {
		go userServices.SendCodeInvitation(uc.emailService, newUser.Email, newUser.FirstName)
	} else {
		go uc.sendInvitation(user, tempPassword)
	}

	return api.OK(user)
}

// assignRoles gives the new member the roles the invitation chose. The user already exists when one is
// refused, so the answer names the role and the row shows up with what it got.
func (uc *UserController) assignRoles(c *gin.Context, tenantID uuid.UUID, user *coreModels.User, roleIDs []uuid.UUID) error {
	requesterID, ok := contextUUID(c, "user_id")
	newUserID, err := uuid.Parse(user.ID)
	if !ok || err != nil {
		return nil
	}
	for _, roleID := range roleIDs {
		if err := uc.userService.AssignRole(c.Request.Context(), tenantID, requesterID, newUserID, roleID); err != nil {
			if message := pgMessage(err); message != "" {
				return api.Fail(coreErrors.StatusOf(err), coreErrors.BuildErrorSingle(c, message))
			}
			return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		}
	}
	return nil
}

// ResendInvitation is POST /users/{id}/invitation: mails the sign-in invitation again to someone of this
// workspace who never signed in (USERS-015 D4).
func (uc *UserController) ResendInvitation(ctx context.Context, in *api.ByID) (*api.Empty, error) {
	c := api.Gin(ctx)
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	target, err := uc.userService.InvitationTarget(c.Request.Context(), requestTenant(c), id)
	if err != nil {
		if message := pgMessage(err); message != "" {
			return nil, api.Fail(coreErrors.StatusOf(err), coreErrors.BuildErrorSingle(c, message))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	go userServices.SendCodeInvitation(uc.emailService, target.Email, target.FirstName)
	return api.Done()
}

// joinTenant adds the new user to the request's workspace with role (member when empty) and
// records the creation in its audit log.
func (uc *UserController) joinTenant(c *gin.Context, tenantID uuid.UUID, user *coreModels.User, role string) {
	requesterID, hasRequester := contextUUID(c, "user_id")
	newUserID, err := uuid.Parse(user.ID)
	if hasRequester && err == nil {
		if role == "" {
			role = tenancyModels.RoleMember
		}
		_ = uc.userService.AssignToTenant(tenantID, requesterID, newUserID, role)
	}

	var performedByPtr, targetIDPtr *uuid.UUID
	if hasRequester {
		performedByPtr = &requesterID
	}
	if err == nil {
		targetIDPtr = &newUserID
	}
	auditSvc := uc.userAuditService
	go func() {
		_ = auditSvc.Record(tenantID, "user.created", targetIDPtr, performedByPtr, nil, "user")
	}()
}

// sendInvitation emails the new user their credentials. Fire-and-forget: a failed email does not
// fail the request.
func (uc *UserController) sendInvitation(user *coreModels.User, tempPassword string) {
	appConf := config.ModularAppConfig.Core
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
		"Title":              "Welcome to " + appConf.AppName,
		"Greeting":           fmt.Sprintf("Welcome, %s!", firstName),
		"Description":        fmt.Sprintf("Your account has been created on %s. Use the credentials below to sign in.", appConf.AppName),
		"Email":              email,
		"TempPassword":       tempPassword,
		"ChangePasswordNote": "For security, please change your password after your first login.",
		"LoginURL":           appConf.FrontendURL + "/login",
		"ButtonText":         "Sign in now",
		"SignOff":            fmt.Sprintf("The %s team", appConf.AppName),
		"AutomatedNote":      "This is an automated message. Please do not reply.",
	}
	_ = uc.emailService.SendEmail(email, fmt.Sprintf("Your %s account is ready", appConf.AppName), templatePath, data)
}

func (uc *UserController) Update(ctx context.Context, in *UpdateUserInput) (*api.Out[*coreModels.User], error) {
	c := api.Gin(ctx)
	var updateUser userModels.UpdateUserDto
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	updateUser.ID = id

	if err := in.JSON.Into(ctx, &updateUser); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserValidationFailed, utils.ExtractValidationError(c, err)))
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
		switch pgMessage(err) {
		case usersErrors.UserNotFound:
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
		case usersErrors.UserUpdateSharedAccount:
			return nil, api.Fail(http.StatusConflict, coreErrors.BuildError(c, err))
		}
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, usersErrors.UserUpdateFailed, err.Error()))
	}

	return api.OK(user)
}

func (uc *UserController) GetStats(ctx context.Context, _ *api.None) (*api.Out[*userModels.UserStatsResponse], error) {
	c := api.Gin(ctx)
	var tenantID uuid.UUID
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			tenantID = tid
		}
	}

	// The caller counts: the list shows them too, marked "· tú" (USERS-015).
	stats, err := uc.userService.GetStats(tenantID, nil)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(stats)
}

func (uc *UserController) ListUsers(ctx context.Context, in *ListUsersInput) (*api.Out[*userModels.UserListResponse], error) {
	c := api.Gin(ctx)
	usersConfig := config.ModularAppConfig.Users

	// Parse pagination parameters
	page, limit := in.Page, in.Limit
	if limit == 0 {
		limit = usersConfig.DefaultPageSize
	}

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
		Page:    page,
		Limit:   limit,
		Search:  in.Search,
		Status:  in.Status,
		Sort:    in.Sort,
		Order:   in.Order,
		Level:   in.Level,
		Pending: in.Pending,
	}
	if in.RoleID != "" {
		roleID, err := uuid.Parse(in.RoleID)
		if err != nil {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		}
		query.RoleID = &roleID
	}
	switch query.Level {
	case "", "portal", "driver", "organization", "web", "admin":
	default:
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, usersErrors.UserValidationFailed))
	}

	// Scope results to the current tenant (set by tenancy middleware)
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			query.TenantID = tid
		}
	}

	response, err := uc.userService.ListUsers(query)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(response)
}

func (uc *UserController) GetUserById(ctx context.Context, in *api.ByID) (*api.Out[*coreModels.User], error) {
	c := api.Gin(ctx)
	userIDStr := in.ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	user, err := uc.userService.GetUserById(requestTenant(c), userID)
	if err != nil {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
	}

	return api.OK(user)
}

func (uc *UserController) SoftDeleteUser(ctx context.Context, in *api.ByID) (*api.Out[SoftDeleteResult], error) {
	c := api.Gin(ctx)
	usersConfig := config.ModularAppConfig.Users

	// Check if user deletion is allowed
	if !usersConfig.AllowUserDeletion {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, usersErrors.UserDeleteNotAllowed))
	}

	userIDStr := in.ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	// Use soft delete if enabled, otherwise permanent delete
	if usersConfig.EnableSoftDelete {
		err = uc.userService.SoftDeleteUser(requestTenant(c), userID)
	} else {
		// If hard delete is implemented, call it here
		err = uc.userService.SoftDeleteUser(requestTenant(c), userID) // TODO: implement HardDeleteUser when needed
	}

	if pgMessage(err) == usersErrors.UserNotFound {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildError(c, err))
	}
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
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

	return api.OK(SoftDeleteResult{Success: true, SoftDelete: usersConfig.EnableSoftDelete})
}

func (uc *UserController) ResetPassword(ctx context.Context, in *api.ByID) (*api.Out[any], error) {
	c := api.Gin(ctx)
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	result, err := uc.userService.ResetPassword(requestTenant(c), id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case "user.not-found",
				"password.reset.account-not-exists",
				"password.reset.token-already-sent":
				return nil, api.Fail(http.StatusOK, nil)
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
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

	return api.OK[any](nil)
}
