package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/config"
	coreModels "josex/web/modules/core/models"
	coreServices "josex/web/modules/core/services"
	coreUtils "josex/web/modules/core/utils"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	tenancyModels "josex/web/modules/tenancy/models"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
)

// emailPattern mirrors auth.private_validate_email so the dry-run preview agrees
// with what the stored procedure will accept at process time.
var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

// usersImportDescriptor is the Users resource plug-in for the generic import
// engine: header signature, per-row validation/processing, and run recording.
type usersImportDescriptor struct {
	userService  userInterfaces.UserService
	auditService userInterfaces.UserAuditService
	emailService coreServices.EmailService
	dbService    coreServices.DatabaseService
}

// NewUsersImportDescriptor builds the Users import descriptor from existing
// services, reusing InsertUser/AssignToTenant/Record. dbService is used only for
// the read-only email-existence preview (no user repository method exposes it).
func NewUsersImportDescriptor(
	userService userInterfaces.UserService,
	auditService userInterfaces.UserAuditService,
	emailService coreServices.EmailService,
	dbService coreServices.DatabaseService,
) importInterfaces.ImportDescriptor {
	return &usersImportDescriptor{
		userService:  userService,
		auditService: auditService,
		emailService: emailService,
		dbService:    dbService,
	}
}

func (d *usersImportDescriptor) Resource() string {
	return "users"
}

func (d *usersImportDescriptor) Columns() []string {
	return []string{"first_name", "last_name", "email", "phone", "birthday"}
}

// Matches recognizes a Users CSV by its header signature: first_name, last_name,
// and email must all be present (phone and birthday are optional).
func (d *usersImportDescriptor) Matches(headers []string) bool {
	required := map[string]bool{"first_name": false, "last_name": false, "email": false}
	for _, header := range headers {
		if _, ok := required[header]; ok {
			required[header] = true
		}
	}
	for _, present := range required {
		if !present {
			return false
		}
	}
	return true
}

func (d *usersImportDescriptor) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}
	fieldErrors, warnings := d.checkFormat(row)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusInvalid
		result.Errors = fieldErrors
		return result
	}

	email := normalizeEmail(row["email"])
	if d.emailExists(email) {
		result.Status = importModels.RowStatusDuplicate
		result.Errors = []string{usersErrors.UserImportEmailDuplicate}
		return result
	}

	result.Status = importModels.RowStatusValid
	return result
}

func (d *usersImportDescriptor) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row}
	fieldErrors, warnings := d.checkFormat(row)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	user, err := d.userService.InsertUser(d.buildDto(row))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Message == "user.create.email.already-exists" {
			result.Status = importModels.RowStatusSkipped
			result.Errors = []string{usersErrors.UserImportEmailDuplicate}
			return result
		}
		result.Status = importModels.RowStatusFailed
		result.Errors = []string{usersErrors.UserImportCreateFailed}
		return result
	}

	d.assignAndAudit(ctx, user)
	d.sendInvitation(ctx, user)

	result.Status = importModels.RowStatusCreated
	return result
}

// RecordRun persists the run summary (D1) as one import.performed audit event
// with the counts as metadata. Best-effort: a failure never fails the import.
func (d *usersImportDescriptor) RecordRun(ctx importModels.ImportContext, meta importModels.RunMeta) {
	payload := map[string]any{
		"resource": meta.Resource,
		"total":    meta.Total,
		"created":  meta.Created,
		"failed":   meta.Failed,
		"skipped":  meta.Skipped,
		"warnings": meta.Warnings,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	metadata := json.RawMessage(raw)
	_ = d.auditService.Record(ctx.TenantID, "import.performed", nil, ctx.PerformedBy, &metadata, "import")
}

// checkFormat validates the required fields and email format, and flags an
// unparseable (but optional) birthday as a non-fatal warning.
func (d *usersImportDescriptor) checkFormat(row map[string]string) ([]string, []string) {
	var fieldErrors []string
	var warnings []string

	if row["first_name"] == "" {
		fieldErrors = append(fieldErrors, usersErrors.UserImportFirstNameRequired)
	}
	if row["last_name"] == "" {
		fieldErrors = append(fieldErrors, usersErrors.UserImportLastNameRequired)
	}

	email := normalizeEmail(row["email"])
	if email == "" {
		fieldErrors = append(fieldErrors, usersErrors.UserImportEmailRequired)
	} else if !emailPattern.MatchString(email) {
		fieldErrors = append(fieldErrors, usersErrors.UserImportEmailInvalid)
	}

	if birthday := row["birthday"]; birthday != "" {
		if _, err := time.Parse("2006-01-02", birthday); err != nil {
			warnings = append(warnings, usersErrors.UserImportBirthdayInvalid)
		}
	}

	return fieldErrors, warnings
}

// emailExists performs a read-only global existence check matching the uniqueness
// InsertUser enforces. It writes nothing, so validate stays a true dry run.
func (d *usersImportDescriptor) emailExists(email string) bool {
	exists := false
	row := d.dbService.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM auth.users WHERE email = LOWER(TRIM($1)))`, email)
	if err := row.Scan(&exists); err != nil {
		return false
	}
	return exists
}

// buildDto maps a CSV row to a CreateUserDto. Imported users get no usable
// password: a random one is generated so the create SP accepts the row, but it
// is never exposed — sign-in is via email OTP (D2/D3). An invalid birthday was
// already surfaced as a warning and is left unset here.
func (d *usersImportDescriptor) buildDto(row map[string]string) userModels.CreateUserDto {
	dto := userModels.CreateUserDto{
		FirstName: row["first_name"],
		LastName:  row["last_name"],
		Email:     normalizeEmail(row["email"]),
		Phone:     row["phone"],
		Password:  coreUtils.GenerateRandomPassword(),
	}
	if birthday := row["birthday"]; birthday != "" {
		if parsed, err := time.Parse("2006-01-02", birthday); err == nil {
			date := coreModels.DateOnly(parsed)
			dto.Birthday = &date
		}
	}
	return dto
}

// assignAndAudit adds the new user to the tenant and records a user.created event.
func (d *usersImportDescriptor) assignAndAudit(ctx importModels.ImportContext, user *coreModels.User) {
	newUserID, err := uuid.Parse(user.ID)
	if err != nil {
		return
	}
	if ctx.PerformedBy != nil {
		_ = d.userService.AssignToTenant(ctx.TenantID, *ctx.PerformedBy, newUserID, tenancyModels.RoleMember)
	}

	auditService := d.auditService
	tenantID := ctx.TenantID
	performedBy := ctx.PerformedBy
	targetID := newUserID
	go func() {
		_ = auditService.Record(tenantID, "user.created", &targetID, performedBy, nil, "import")
	}()
}

// sendInvitation fires the OTP-login invitation email unless the caller disabled
// it via options.send_invitation. Fire-and-forget: email failure never fails the row.
func (d *usersImportDescriptor) sendInvitation(ctx importModels.ImportContext, user *coreModels.User) {
	if !ctx.Options.BoolOr("send_invitation", true) {
		return
	}

	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	if email == "" {
		return
	}
	firstName := ""
	if user.FirstName != nil {
		firstName = *user.FirstName
	}

	appConf := config.ModularAppConfig.Core
	subject := fmt.Sprintf("You're invited to %s", appConf.AppName)
	data := map[string]string{
		"Title":         subject,
		"Greeting":      fmt.Sprintf("Welcome, %s!", firstName),
		"Description":   fmt.Sprintf("An account has been created for you on %s. Sign in with a one-time email code — no password required.", appConf.AppName),
		"Email":         email,
		"ButtonText":    "Sign in with an email code",
		"LoginURL":      fmt.Sprintf("%s/login", appConf.FrontendURL),
		"SignOff":       fmt.Sprintf("The %s team", appConf.AppName),
		"AutomatedNote": "This is an automated message. Please do not reply.",
	}
	templatePath := coreServices.GetTemplatePath("users", "user-import-invitation.html")

	emailService := d.emailService
	go func() {
		_ = emailService.SendEmail(email, subject, templatePath, data)
	}()
}

// normalizeEmail trims and lowercases an email, matching the create SP's handling.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
