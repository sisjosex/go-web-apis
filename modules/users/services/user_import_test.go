package services

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage/storagetest"
	importModels "josex/web/modules/import/models"
	usersErrors "josex/web/modules/users/errors"
)

// newTestDescriptor builds the descriptor with only the dependency checkFormat
// and buildDto actually use: the media service, on the test media bucket.
// No database, no HTTP route — this is a white-box test of the field mapping (D4).
func newTestDescriptor(t *testing.T) *usersImportDescriptor {
	t.Helper()
	return &usersImportDescriptor{mediaService: coreServices.NewMediaService(storagetest.Media(t))}
}

// completeRow is a row filling every column the descriptor declares except
// `roles`, whose resolution needs a database — it has its own cases below and an
// integration test in modules/users/tests.
func completeRow() map[string]string {
	return map[string]string{
		"first_name":      "Ana",
		"last_name":       "Ruiz",
		"email":           "  Ana.Ruiz@Example.COM ",
		"phone":           "+34600111222",
		"birthday":        "1990-04-17",
		"bio":             "Product designer",
		"website_url":     "https://ana.example.com",
		"profile_picture": "ana.png",
	}
}

func contextWithImages(images map[string][]byte) importModels.ImportContext {
	return importModels.ImportContext{Images: importModels.ImportImages(images)}
}

func TestCheckFormat_CompleteRowHasNoErrorsOrWarnings(t *testing.T) {
	descriptor := newTestDescriptor(t)
	ctx := contextWithImages(map[string][]byte{"ana.png": []byte("ana-bytes")})

	fieldErrors, warnings, _ := descriptor.checkFormat(ctx, completeRow())

	if len(fieldErrors) != 0 {
		t.Errorf("expected no errors, got %v", fieldErrors)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestBuildDto_CompleteRowMapsEveryColumn(t *testing.T) {
	descriptor := newTestDescriptor(t)
	ctx := contextWithImages(map[string][]byte{"ana.png": []byte("ana-bytes")})

	dto, warnings := descriptor.buildDto(ctx, completeRow())

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if dto.FirstName != "Ana" || dto.LastName != "Ruiz" {
		t.Errorf("expected the names to map, got %q %q", dto.FirstName, dto.LastName)
	}
	if dto.Email != "ana.ruiz@example.com" {
		t.Errorf("expected the email trimmed and lowercased, got %q", dto.Email)
	}
	if dto.Phone != "+34600111222" {
		t.Errorf("expected the phone to map, got %q", dto.Phone)
	}
	if dto.Bio != "Product designer" {
		t.Errorf("expected the bio to map, got %q", dto.Bio)
	}
	if dto.WebsiteUrl != "https://ana.example.com" {
		t.Errorf("expected the website to map, got %q", dto.WebsiteUrl)
	}
	if dto.Birthday == nil {
		t.Error("expected the birthday to map")
	}
	if dto.Password == "" {
		t.Error("expected a generated password so the create SP accepts the row")
	}

	if !strings.Contains(dto.ProfilePictureUrl, "/avatars/") {
		t.Fatalf("expected a stored picture URL, got %q", dto.ProfilePictureUrl)
	}
	res, err := http.Get(dto.ProfilePictureUrl)
	if err != nil {
		t.Fatalf("GET the picture: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("expected the picture served as image/png, got %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
}

func TestBuildDto_BlankOptionalColumnsStayUnset(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := map[string]string{
		"first_name": "Ana",
		"last_name":  "Ruiz",
		"email":      "ana@example.com",
	}

	fieldErrors, warnings, _ := descriptor.checkFormat(contextWithImages(nil), row)
	dto, pictureWarnings := descriptor.buildDto(contextWithImages(nil), row)

	if len(fieldErrors) != 0 || len(warnings) != 0 || len(pictureWarnings) != 0 {
		t.Fatalf("a name-and-email row is clean: errors=%v warnings=%v picture=%v",
			fieldErrors, warnings, pictureWarnings)
	}
	if dto.Phone != "" || dto.Bio != "" || dto.WebsiteUrl != "" || dto.ProfilePictureUrl != "" {
		t.Errorf("expected the blank optionals to stay unset, got %+v", dto)
	}
	if dto.Birthday != nil {
		t.Error("expected no birthday")
	}
}

func TestCheckFormat_MissingRequiredFieldsAreErrors(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := map[string]string{"first_name": "", "last_name": "", "email": "not-an-email"}

	fieldErrors, _, _ := descriptor.checkFormat(contextWithImages(nil), row)

	for _, expected := range []string{
		usersErrors.UserImportFirstNameRequired,
		usersErrors.UserImportLastNameRequired,
		usersErrors.UserImportEmailInvalid,
	} {
		if !slices.Contains(fieldErrors, expected) {
			t.Errorf("expected %s in %v", expected, fieldErrors)
		}
	}
}

func TestCheckFormat_InvalidWebsiteWarnsAndTheFieldIsDropped(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := completeRow()
	row["website_url"] = "ana.example.com" // no scheme
	row["profile_picture"] = ""

	fieldErrors, warnings, _ := descriptor.checkFormat(contextWithImages(nil), row)
	dto, _ := descriptor.buildDto(contextWithImages(nil), row)

	if len(fieldErrors) != 0 {
		t.Fatalf("a bad website is a warning, not an error: %v", fieldErrors)
	}
	if !slices.Contains(warnings, usersErrors.UserImportWebsiteInvalid) {
		t.Errorf("expected %s in %v", usersErrors.UserImportWebsiteInvalid, warnings)
	}
	if dto.WebsiteUrl != "" {
		t.Errorf("expected the invalid website left unset, got %q", dto.WebsiteUrl)
	}
	if dto.Email == "" {
		t.Error("expected the row to still be creatable")
	}
}

func TestCheckFormat_InvalidBirthdayWarnsAndTheFieldIsDropped(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := completeRow()
	row["birthday"] = "17/04/1990"
	row["profile_picture"] = ""

	fieldErrors, warnings, _ := descriptor.checkFormat(contextWithImages(nil), row)
	dto, _ := descriptor.buildDto(contextWithImages(nil), row)

	if len(fieldErrors) != 0 {
		t.Fatalf("a bad birthday is a warning, not an error: %v", fieldErrors)
	}
	if !slices.Contains(warnings, usersErrors.UserImportBirthdayInvalid) {
		t.Errorf("expected %s in %v", usersErrors.UserImportBirthdayInvalid, warnings)
	}
	if dto.Birthday != nil {
		t.Error("expected the invalid birthday left unset")
	}
}

func TestCheckFormat_PictureAbsentFromArchiveWarnsAndTheRowIsStillBuilt(t *testing.T) {
	descriptor := newTestDescriptor(t)
	ctx := contextWithImages(map[string][]byte{"someone-else.png": []byte("bytes")})
	row := completeRow()

	fieldErrors, warnings, _ := descriptor.checkFormat(ctx, row)
	dto, pictureWarnings := descriptor.buildDto(ctx, row)

	if len(fieldErrors) != 0 {
		t.Fatalf("a missing picture is a warning, not an error: %v", fieldErrors)
	}
	if !slices.Contains(warnings, usersErrors.UserImportImageMissing) {
		t.Errorf("expected %s in %v", usersErrors.UserImportImageMissing, warnings)
	}
	if len(pictureWarnings) != 0 {
		t.Errorf("expected no second warning from buildDto, got %v", pictureWarnings)
	}
	if dto.ProfilePictureUrl != "" {
		t.Errorf("expected no picture URL, got %q", dto.ProfilePictureUrl)
	}
	if dto.Email == "" {
		t.Error("expected the row to still be creatable")
	}
}

func TestCheckFormat_PictureIsMatchedCaseInsensitively(t *testing.T) {
	descriptor := newTestDescriptor(t)
	ctx := contextWithImages(map[string][]byte{"ana.png": []byte("ana-bytes")})
	row := completeRow()
	row["profile_picture"] = " Ana.PNG "

	_, warnings, _ := descriptor.checkFormat(ctx, row)
	dto, _ := descriptor.buildDto(ctx, row)

	if len(warnings) != 0 {
		t.Fatalf("expected the entry to be found, got %v", warnings)
	}
	if dto.ProfilePictureUrl == "" {
		t.Error("expected the picture to be stored")
	}
}

func TestParseRoleNames_SplitsTrimsAndDeduplicates(t *testing.T) {
	names := parseRoleNames(" Sales ;; support;SALES;  ")

	expected := []string{"Sales", "support"}
	if !slices.Equal(names, expected) {
		t.Errorf("expected %v — trimmed, blanks dropped, first spelling kept — got %v", expected, names)
	}
}

func TestParseRoleNames_BlankCellYieldsNothing(t *testing.T) {
	for _, cell := range []string{"", "   ", ";", " ; ; "} {
		if names := parseRoleNames(cell); len(names) != 0 {
			t.Errorf("expected %q to yield no names, got %v", cell, names)
		}
	}
}

// A role this tenant does not have is a field error, not a warning (D3), and the
// code carries the operator's own spelling so the message can quote it back.
// newTestDescriptor has no dbService, so every name here is unresolvable — the
// resolution itself is covered by the integration test.
func TestCheckFormat_UnresolvableRoleIsAFieldErrorNamingTheRole(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := completeRow()
	row["profile_picture"] = ""
	row["roles"] = "SampleRole"

	fieldErrors, warnings, roleIDs := descriptor.checkFormat(contextWithImages(nil), row)

	if !slices.Contains(fieldErrors, usersErrors.UserImportRoleUnknown+errorParamSeparator+"SampleRole") {
		t.Errorf("expected the code to carry the role name, got %v", fieldErrors)
	}
	if len(warnings) != 0 {
		t.Errorf("an unknown role rejects the row, it does not warn: %v", warnings)
	}
	if len(roleIDs) != 0 {
		t.Errorf("expected no resolved ids, got %v", roleIDs)
	}
}

func TestCheckFormat_BlankRolesCellResolvesNothingAndIsClean(t *testing.T) {
	descriptor := newTestDescriptor(t)
	row := completeRow()
	row["profile_picture"] = ""
	row["roles"] = "  "

	fieldErrors, _, roleIDs := descriptor.checkFormat(contextWithImages(nil), row)

	if len(fieldErrors) != 0 {
		t.Errorf("a blank roles cell is not an error: %v", fieldErrors)
	}
	if len(roleIDs) != 0 {
		t.Errorf("expected no resolved ids, got %v", roleIDs)
	}
}

func TestColumns_RolesIsDeclaredOptionalWithAHint(t *testing.T) {
	descriptor := newTestDescriptor(t)

	index := slices.IndexFunc(descriptor.Columns(), func(spec importModels.ColumnSpec) bool {
		return spec.Key == "roles"
	})
	if index == -1 {
		t.Fatalf("expected a roles column in %v", descriptor.Columns())
	}

	spec := descriptor.Columns()[index]
	if spec.Required {
		t.Error("expected roles to be optional")
	}
	if spec.Type != importModels.ColumnTypeText {
		t.Errorf("expected a text column, got %q", spec.Type)
	}
	if spec.Format != rolesFormat {
		t.Errorf("expected the separator hint %q, got %q", rolesFormat, spec.Format)
	}
}

func TestColumns_CarryTheProfileFieldsAndTheirTypes(t *testing.T) {
	descriptor := newTestDescriptor(t)

	specs := map[string]importModels.ColumnSpec{}
	for _, spec := range descriptor.Columns() {
		specs[spec.Key] = spec
	}

	for key, expected := range map[string]importModels.ColumnType{
		"first_name":      importModels.ColumnTypeText,
		"email":           importModels.ColumnTypeEmail,
		"birthday":        importModels.ColumnTypeDate,
		"bio":             importModels.ColumnTypeText,
		"website_url":     importModels.ColumnTypeURL,
		"profile_picture": importModels.ColumnTypeImage,
	} {
		spec, ok := specs[key]
		if !ok {
			t.Errorf("expected a %s column", key)
			continue
		}
		if spec.Type != expected {
			t.Errorf("expected %s typed %s, got %s", key, expected, spec.Type)
		}
	}

	if !specs["email"].Required || specs["bio"].Required {
		t.Error("expected email required and bio optional")
	}
	if specs["website_url"].Pattern == "" {
		t.Error("expected the website column to publish its pattern so the wizard checks what the API checks")
	}
}
