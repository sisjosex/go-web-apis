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

// websitePattern is the single definition of a valid website_url: it is checked
// here on import and published on the column spec so the wizard and the admin
// modal reject exactly the same strings (D3).
var websitePattern = regexp.MustCompile(`^https?://[^\s/$.?#][^\s]*$`)

// birthdayLayout is the Go layout accepted for the birthday column;
// birthdayFormat is the same shape written for humans (template placeholder,
// schema hint), so both sides of the contract move together.
const (
	birthdayLayout = "2006-01-02"
	birthdayFormat = "YYYY-MM-DD"

	// imageNameFormat tells the operator what a profile_picture cell holds: the
	// bare filename of an entry in the companion archive, not a URL or a path.
	imageNameFormat = "filename.jpg"

	// avatarsCategory is the media directory imported pictures are written to.
	avatarsCategory = "avatars"

	// roleSeparator splits the roles cell. A semicolon, not a comma, so the cell
	// survives a hand-edit in a spreadsheet without needing to be quoted.
	roleSeparator = ";"
	// rolesFormat is the same shape written for humans (template placeholder,
	// schema hint), so both sides of the contract move together.
	rolesFormat = "Sales;Support"

	// errorParamSeparator appends a value to a row error code, for the codes whose
	// message quotes back what the cell said: "users.import.role-unknown|Sales".
	// The client splits on it and interpolates the tail; a code without it is
	// translated as-is, so every existing code is unaffected.
	errorParamSeparator = "|"
)

// usersImportDescriptor is the Users resource plug-in for the generic import
// engine: header signature, per-row validation/processing, and run recording.
type usersImportDescriptor struct {
	userService  userInterfaces.UserService
	auditService userInterfaces.UserAuditService
	emailService coreServices.EmailService
	dbService    coreServices.DatabaseService
	mediaService coreServices.MediaService
}

// NewUsersImportDescriptor builds the Users import descriptor from existing
// services, reusing InsertUser/AssignToTenant/Record. dbService is used only for
// the read-only email-existence preview (no user repository method exposes it);
// mediaService stores the picture a row names in the companion image archive.
func NewUsersImportDescriptor(
	userService userInterfaces.UserService,
	auditService userInterfaces.UserAuditService,
	emailService coreServices.EmailService,
	dbService coreServices.DatabaseService,
	mediaService coreServices.MediaService,
) importInterfaces.ImportDescriptor {
	return &usersImportDescriptor{
		userService:  userService,
		auditService: auditService,
		emailService: emailService,
		dbService:    dbService,
		mediaService: mediaService,
	}
}

func (d *usersImportDescriptor) Resource() string {
	return "users"
}

// Columns is the canonical column order with the type and validation a client
// needs to edit each cell. The email pattern is the API's own so the wizard
// rejects exactly what checkFormat would.
func (d *usersImportDescriptor) Columns() []importModels.ColumnSpec {
	return []importModels.ColumnSpec{
		{Key: "first_name", Type: importModels.ColumnTypeText, Required: true},
		{Key: "last_name", Type: importModels.ColumnTypeText, Required: true},
		{Key: "email", Type: importModels.ColumnTypeEmail, Required: true, Pattern: emailPattern.String()},
		{Key: "phone", Type: importModels.ColumnTypePhone},
		{Key: "birthday", Type: importModels.ColumnTypeDate, Format: birthdayFormat},
		{Key: "bio", Type: importModels.ColumnTypeText},
		{Key: "website_url", Type: importModels.ColumnTypeURL, Pattern: websitePattern.String()},
		{Key: "profile_picture", Type: importModels.ColumnTypeImage, Format: imageNameFormat},
		{Key: "roles", Type: importModels.ColumnTypeText, Format: rolesFormat},
	}
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
	fieldErrors, warnings, _ := d.checkFormat(ctx, row)
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
	fieldErrors, warnings, roleIDs := d.checkFormat(ctx, row)
	result.Warnings = warnings

	if len(fieldErrors) > 0 {
		result.Status = importModels.RowStatusFailed
		result.Errors = fieldErrors
		return result
	}

	dto, pictureWarnings := d.buildDto(ctx, row)
	result.Warnings = append(result.Warnings, pictureWarnings...)

	user, err := d.userService.InsertUser(dto)
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

	result.Warnings = append(result.Warnings, d.assignAndAudit(ctx, user, roleIDs)...)
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

// checkFormat validates the required fields and email format, and flags the
// optional fields it cannot use as non-fatal warnings: an unparseable birthday,
// a malformed website_url (D3), and a picture named but absent from the archive
// (D6). In every warning case the row is still created, with the field unset.
//
// The roles cell is the exception: a name this tenant does not have is a field
// error, not a warning, so the row is rejected instead of being created with
// less access than the sheet asked for (USERS-010 D3). It returns the role ids it
// resolved so ProcessRow grants them without querying again.
func (d *usersImportDescriptor) checkFormat(ctx importModels.ImportContext, row map[string]string) ([]string, []string, []uuid.UUID) {
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
		if _, err := time.Parse(birthdayLayout, birthday); err != nil {
			warnings = append(warnings, usersErrors.UserImportBirthdayInvalid)
		}
	}

	if website := row["website_url"]; website != "" && !websitePattern.MatchString(website) {
		warnings = append(warnings, usersErrors.UserImportWebsiteInvalid)
	}

	if picture := row["profile_picture"]; picture != "" {
		if _, found := ctx.Images.Lookup(picture); !found {
			warnings = append(warnings, usersErrors.UserImportImageMissing)
		}
	}

	roleIDs, unknownRoles := d.resolveRoles(ctx.TenantID, parseRoleNames(row["roles"]))
	for _, name := range unknownRoles {
		fieldErrors = append(fieldErrors, usersErrors.UserImportRoleUnknown+errorParamSeparator+name)
	}

	return fieldErrors, warnings, roleIDs
}

// parseRoleNames splits a roles cell into the distinct names it holds, trimmed
// and de-duplicated case-insensitively. It keeps the first spelling the operator
// typed, because that is the spelling an error message quotes back to them.
func parseRoleNames(cell string) []string {
	var names []string
	seen := make(map[string]bool)

	for _, part := range strings.Split(cell, roleSeparator) {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}

	return names
}

// resolveRoles maps role names to their ids within one tenant, matched
// case-insensitively (tenancy.roles is UNIQUE(tenant_id, name)). It returns the
// ids it resolved and the names it could not, in the operator's own spelling.
//
// The read reaches another module's schema through dbService rather than by
// importing tenancy's repository. It needs no to_regclass guard: /import only
// registers inside the tenancy-enabled block, so this code never runs against a
// database without the schema.
func (d *usersImportDescriptor) resolveRoles(tenantID uuid.UUID, names []string) ([]uuid.UUID, []string) {
	if len(names) == 0 {
		return nil, nil
	}
	if d.dbService == nil {
		return nil, names
	}

	lowered := make([]string, 0, len(names))
	for _, name := range names {
		lowered = append(lowered, strings.ToLower(name))
	}

	query := `
        SELECT r.id, LOWER(r.name)
        FROM tenancy.roles r
        WHERE r.tenant_id = $1
          AND LOWER(r.name) = ANY($2)
    `
	rows, err := d.dbService.Query(context.Background(), query, tenantID, lowered)
	if err != nil {
		return nil, names
	}
	defer rows.Close()

	found := make(map[string]uuid.UUID, len(names))
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, names
		}
		found[name] = id
	}
	if rows.Err() != nil {
		return nil, names
	}

	// Walk the requested names, not the map, so the ids keep the sheet's order and
	// every miss is reported once, spelled as the operator wrote it.
	var roleIDs []uuid.UUID
	var unknown []string
	for _, name := range names {
		if id, ok := found[strings.ToLower(name)]; ok {
			roleIDs = append(roleIDs, id)
			continue
		}
		unknown = append(unknown, name)
	}

	return roleIDs, unknown
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
// is never exposed — sign-in is via email OTP (D2/D3).
//
// Any optional field checkFormat already warned about is simply left unset here:
// an invalid birthday, a malformed website_url, and a picture the archive does
// not carry. It returns the extra warnings raised while writing the picture,
// which is the only step that can fail after validation passed.
func (d *usersImportDescriptor) buildDto(ctx importModels.ImportContext, row map[string]string) (userModels.CreateUserDto, []string) {
	dto := userModels.CreateUserDto{
		FirstName: row["first_name"],
		LastName:  row["last_name"],
		Email:     normalizeEmail(row["email"]),
		Phone:     row["phone"],
		Bio:       row["bio"],
		Password:  coreUtils.GenerateRandomPassword(),
	}
	if birthday := row["birthday"]; birthday != "" {
		if parsed, err := time.Parse(birthdayLayout, birthday); err == nil {
			date := coreModels.DateOnly(parsed)
			dto.Birthday = &date
		}
	}
	if website := row["website_url"]; website != "" && websitePattern.MatchString(website) {
		dto.WebsiteUrl = website
	}

	var warnings []string
	if url, warning := d.storePicture(ctx, row["profile_picture"]); warning != "" {
		warnings = append(warnings, warning)
	} else {
		dto.ProfilePictureUrl = url
	}

	return dto, warnings
}

// storePicture writes the archive entry a row names and returns its public URL.
// A blank cell, an entry the archive does not carry (already warned by
// checkFormat), or a missing media service are all "nothing to store" — only a
// failed write raises a new warning.
func (d *usersImportDescriptor) storePicture(ctx importModels.ImportContext, filename string) (string, string) {
	if filename == "" || d.mediaService == nil {
		return "", ""
	}

	content, found := ctx.Images.Lookup(filename)
	if !found {
		return "", ""
	}

	url, err := d.mediaService.Save(avatarsCategory, coreServices.MediaFile{Filename: filename, Content: content})
	if err != nil {
		return "", usersErrors.UserImportImageInvalid
	}
	return url, ""
}

// assignAndAudit adds the new user to the tenant, grants the roles the row named,
// and records a user.created event. It returns one warning per role it could not
// grant — the user exists by then, so a failed grant degrades the row instead of
// failing it.
func (d *usersImportDescriptor) assignAndAudit(ctx importModels.ImportContext, user *coreModels.User, roleIDs []uuid.UUID) []string {
	newUserID, err := uuid.Parse(user.ID)
	if err != nil {
		return nil
	}

	var warnings []string
	if ctx.PerformedBy != nil {
		_ = d.userService.AssignToTenant(ctx.TenantID, *ctx.PerformedBy, newUserID, tenancyModels.RoleMember)
		// Strictly after the membership: sp_assign_user_role rejects a user who is
		// not yet an active member of the tenant.
		warnings = d.grantRoles(ctx, newUserID, roleIDs)
	}

	auditService := d.auditService
	tenantID := ctx.TenantID
	performedBy := ctx.PerformedBy
	targetID := newUserID
	go func() {
		_ = auditService.Record(tenantID, "user.created", &targetID, performedBy, nil, "import")
	}()

	return warnings
}

// grantRoles assigns each already-resolved role to the new member through
// tenancy.sp_assign_user_role, which is idempotent and validates that the actor
// may grant it. A failure here is reported, not swallowed: the row's user is
// already created, so the operator has to know the grant did not land.
func (d *usersImportDescriptor) grantRoles(ctx importModels.ImportContext, userID uuid.UUID, roleIDs []uuid.UUID) []string {
	if len(roleIDs) == 0 || d.dbService == nil || ctx.PerformedBy == nil {
		return nil
	}

	query := `SELECT tenancy.sp_assign_user_role($1, $2, $3, $4)`

	var warnings []string
	for _, roleID := range roleIDs {
		assigned := false
		row := d.dbService.QueryRow(context.Background(), query, ctx.TenantID, *ctx.PerformedBy, userID, roleID)
		if err := row.Scan(&assigned); err != nil || !assigned {
			warnings = append(warnings, usersErrors.UserImportRoleAssignFailed)
		}
	}

	return warnings
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
