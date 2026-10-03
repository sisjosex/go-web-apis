package services

import (
	"context"
	"strconv"
	"strings"

	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	trackingInterfaces "josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The CSV imports of the tracking module (APP-009): what a tenant loads by the dozen when it starts —
// its vehicles, stops, riders and document policy — without typing them one form at a time. Each row
// goes through the same service call the screens use, so a row is refused for the same reasons a form
// is. A carrier or an organization is named, not referenced by id, and may be left empty when the
// tenant has only one; the lookups are read once per run into ImportContext.Scratch.

// Row error codes, translated by the app under tracking.import.*.
const (
	importRequired           = "tracking.import.required"
	importNumberInvalid      = "tracking.import.number-invalid"
	importTypeInvalid        = "tracking.import.type-invalid"
	importCompanyUnknown     = "tracking.import.company-unknown"
	importCompanyRequired    = "tracking.import.company-required"
	importOrgUnknown         = "tracking.import.organization-unknown"
	importOrgRequired        = "tracking.import.organization-required"
	importPlateDuplicate     = "tracking.import.plate-duplicate"
	importPlaceDuplicate     = "tracking.import.place-duplicate"
	importRiderDuplicate     = "tracking.import.rider-duplicate"
	importCodeDuplicate      = "tracking.import.code-duplicate"
	importCoordinatesInvalid = "tracking.import.coordinates-invalid"
	importCreateFailed       = "tracking.import.create-failed"

	// errorParam joins a code and the value it is about, as the wizard reads it.
	errorParam = "|"
	// importLookupPage is how many carriers or organizations a run reads: a tenant's whole list.
	importLookupPage = 100
)

// ── shared ────────────────────────────────────────────────────────────────

func cell(row map[string]string, key string) string {
	return strings.TrimSpace(row[key])
}

func optionalCell(row map[string]string, key string) *string {
	if value := cell(row, key); value != "" {
		return &value
	}
	return nil
}

func hasAll(headers []string, keys ...string) bool {
	set := map[string]bool{}
	for _, h := range headers {
		set[h] = true
	}
	for _, k := range keys {
		if !set[k] {
			return false
		}
	}
	return true
}

func columnsOf(specs ...importModels.ColumnSpec) []importModels.ColumnSpec { return specs }

func textColumn(key string, required bool) importModels.ColumnSpec {
	return importModels.ColumnSpec{Key: key, Type: importModels.ColumnTypeText, Required: required}
}

func numberColumn(key string, required bool) importModels.ColumnSpec {
	return importModels.ColumnSpec{Key: key, Type: importModels.ColumnTypeNumber, Required: required}
}

// scratchSet is a per-run set of keys the rows above have claimed, so a file repeating itself is
// caught on the dry run, before anything is written.
func scratchSet(ctx importModels.ImportContext, name string) map[string]bool {
	if set, ok := ctx.Scratch[name].(map[string]bool); ok {
		return set
	}
	set := map[string]bool{}
	if ctx.Scratch != nil {
		ctx.Scratch[name] = set
	}
	return set
}

// namedIDs resolves a name to an id against a list read once per run; an empty name is the only
// entry when there is exactly one.
type namedIDs struct {
	byName map[string]uuid.UUID
	only   *uuid.UUID
}

func (n namedIDs) resolve(name, unknownCode, requiredCode string) (uuid.UUID, string) {
	if name == "" {
		if n.only != nil {
			return *n.only, ""
		}
		return uuid.Nil, requiredCode
	}
	if id, ok := n.byName[strings.ToLower(name)]; ok {
		return id, ""
	}
	return uuid.Nil, unknownCode + errorParam + name
}

func newNamedIDs(names map[string]uuid.UUID) namedIDs {
	n := namedIDs{byName: map[string]uuid.UUID{}}
	for name, id := range names {
		n.byName[strings.ToLower(name)] = id
	}
	if len(names) == 1 {
		for _, id := range names {
			only := id
			n.only = &only
		}
	}
	return n
}

func companiesOf(ctx importModels.ImportContext, svc trackingInterfaces.TrackingService) namedIDs {
	if cached, ok := ctx.Scratch["tracking.companies"].(namedIDs); ok {
		return cached
	}
	names := map[string]uuid.UUID{}
	if res, err := svc.ListCompanies(ctx.Context(), ctx.TenantID, models.ListCompaniesQuery{Page: 1, PageSize: importLookupPage}); err == nil {
		for _, c := range res.Companies {
			names[c.Name] = c.ID
		}
	}
	n := newNamedIDs(names)
	if ctx.Scratch != nil {
		ctx.Scratch["tracking.companies"] = n
	}
	return n
}

// organizationsOf also keeps each organization's kind: a school's riders are students.
func organizationsOf(ctx importModels.ImportContext, svc trackingInterfaces.TrackingService) (namedIDs, map[uuid.UUID]string) {
	if cached, ok := ctx.Scratch["tracking.organizations"].(namedIDs); ok {
		kinds, _ := ctx.Scratch["tracking.organization-kinds"].(map[uuid.UUID]string)
		return cached, kinds
	}
	names := map[string]uuid.UUID{}
	kinds := map[uuid.UUID]string{}
	if res, err := svc.ListOrganizations(ctx.Context(), ctx.TenantID, models.ListOrganizationsQuery{Page: 1, PageSize: importLookupPage}); err == nil {
		for _, o := range res.Organizations {
			names[o.Name] = o.ID
			kinds[o.ID] = o.Kind
		}
	}
	n := newNamedIDs(names)
	if ctx.Scratch != nil {
		ctx.Scratch["tracking.organizations"] = n
		ctx.Scratch["tracking.organization-kinds"] = kinds
	}
	return n, kinds
}

// rowResult is a row's outcome: invalid on a dry run, failed on the real one, when it has errors.
func rowResult(line int, row map[string]string, errs []string, process bool) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row, Errors: errs}
	switch {
	case len(errs) > 0 && process:
		result.Status = importModels.RowStatusFailed
	case len(errs) > 0:
		result.Status = importModels.RowStatusInvalid
	case process:
		result.Status = importModels.RowStatusCreated
	default:
		result.Status = importModels.RowStatusValid
	}
	return result
}

func duplicate(line int, row map[string]string, code string, process bool) importModels.RowResult {
	result := importModels.RowResult{Line: line, Data: row, Errors: []string{code}, Status: importModels.RowStatusDuplicate}
	if process {
		result.Status = importModels.RowStatusSkipped
	}
	return result
}

func parseNumber(value string, bits int) (*float64, bool) {
	if value == "" {
		return nil, true
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), bits)
	if err != nil {
		return nil, false
	}
	return &n, true
}

// ── vehicles ─────────────────────────────────────────────────────────────

type vehiclesImport struct {
	svc trackingInterfaces.TrackingService
}

// NewVehiclesImportDescriptor imports a carrier's fleet: plate, type, seats and what identifies it.
func NewVehiclesImportDescriptor(svc trackingInterfaces.TrackingService) importInterfaces.ImportDescriptor {
	return &vehiclesImport{svc: svc}
}

func (d *vehiclesImport) Resource() string { return "tracking_vehicles" }

func (d *vehiclesImport) Columns() []importModels.ColumnSpec {
	return columnsOf(textColumn("plate", true), numberColumn("seats", true), textColumn("type", false),
		textColumn("company", false), textColumn("brand", false), textColumn("model", false), numberColumn("year", false))
}

func (d *vehiclesImport) Matches(headers []string) bool { return hasAll(headers, "plate", "seats") }

func (d *vehiclesImport) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, false)
}

func (d *vehiclesImport) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, true)
}

func (d *vehiclesImport) RecordRun(importModels.ImportContext, importModels.RunMeta) {}

func (d *vehiclesImport) run(ctx importModels.ImportContext, line int, row map[string]string, process bool) importModels.RowResult {
	dto, errs := d.dto(ctx, row)
	if len(errs) > 0 {
		return rowResult(line, row, errs, process)
	}
	seen := scratchSet(ctx, "tracking.plates")
	if seen[dto.PlateNumber] || d.plateTaken(ctx.Context(), ctx, dto.PlateNumber) {
		return duplicate(line, row, importPlateDuplicate+errorParam+dto.PlateNumber, process)
	}
	seen[dto.PlateNumber] = true
	if process {
		if _, err := d.svc.CreateVehicle(ctx.Context(), ctx.TenantID, dto); err != nil {
			return rowResult(line, row, []string{importCreateFailed}, true)
		}
	}
	return rowResult(line, row, nil, process)
}

func (d *vehiclesImport) dto(ctx importModels.ImportContext, row map[string]string) (*models.CreateVehicleDto, []string) {
	var errs []string
	plate := strings.ToUpper(cell(row, "plate"))
	if len(plate) < 3 {
		errs = append(errs, importRequired+errorParam+"plate")
	}
	seats, ok := parseNumber(cell(row, "seats"), 32)
	if !ok || seats == nil || *seats < 1 || *seats > 200 {
		errs = append(errs, importNumberInvalid+errorParam+"seats")
	}
	kind := strings.ToLower(cell(row, "type"))
	if kind == "" {
		kind = "bus"
	}
	if kind != "bus" && kind != "van" && kind != "car" {
		errs = append(errs, importTypeInvalid+errorParam+kind)
	}
	companyID, companyErr := companiesOf(ctx, d.svc).resolve(cell(row, "company"), importCompanyUnknown, importCompanyRequired)
	if companyErr != "" {
		errs = append(errs, companyErr)
	}
	year, okYear := parseNumber(cell(row, "year"), 32)
	if !okYear {
		errs = append(errs, importNumberInvalid+errorParam+"year")
	}
	if len(errs) > 0 {
		return nil, errs
	}
	dto := &models.CreateVehicleDto{
		CompanyID: companyID, PlateNumber: plate, VehicleType: kind, Capacity: int32(*seats), Status: "active",
		Brand: optionalCell(row, "brand"), Model: optionalCell(row, "model"),
	}
	if year != nil {
		y := int32(*year)
		dto.Year = &y
	}
	return dto, nil
}

func (d *vehiclesImport) plateTaken(c context.Context, ctx importModels.ImportContext, plate string) bool {
	res, err := d.svc.ListVehicles(c, ctx.TenantID, models.ListVehiclesQuery{Search: plate, Page: 1, PageSize: 20})
	if err != nil {
		return false
	}
	for _, v := range res.Vehicles {
		if strings.EqualFold(v.PlateNumber, plate) {
			return true
		}
	}
	return false
}

// ── stop places ──────────────────────────────────────────────────────────

type stopPlacesImport struct {
	svc trackingInterfaces.TrackingService
}

// NewStopPlacesImportDescriptor imports the places routes call at, each with its point.
func NewStopPlacesImportDescriptor(svc trackingInterfaces.TrackingService) importInterfaces.ImportDescriptor {
	return &stopPlacesImport{svc: svc}
}

func (d *stopPlacesImport) Resource() string { return "tracking_stop_places" }

func (d *stopPlacesImport) Columns() []importModels.ColumnSpec {
	return columnsOf(textColumn("stop", true), numberColumn("latitude", true), numberColumn("longitude", true), textColumn("address", false))
}

func (d *stopPlacesImport) Matches(headers []string) bool {
	return hasAll(headers, "stop", "latitude", "longitude")
}

func (d *stopPlacesImport) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, false)
}

func (d *stopPlacesImport) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, true)
}

func (d *stopPlacesImport) RecordRun(importModels.ImportContext, importModels.RunMeta) {}

func (d *stopPlacesImport) run(ctx importModels.ImportContext, line int, row map[string]string, process bool) importModels.RowResult {
	name := cell(row, "stop")
	var errs []string
	if len(name) < 2 {
		errs = append(errs, importRequired+errorParam+"stop")
	}
	lat, okLat := parseNumber(cell(row, "latitude"), 64)
	lng, okLng := parseNumber(cell(row, "longitude"), 64)
	if !okLat || !okLng || lat == nil || lng == nil || *lat < -90 || *lat > 90 || *lng < -180 || *lng > 180 {
		errs = append(errs, importCoordinatesInvalid)
	}
	if len(errs) > 0 {
		return rowResult(line, row, errs, process)
	}
	key := strings.ToLower(name)
	seen := scratchSet(ctx, "tracking.places")
	if seen[key] || d.nameTaken(ctx, name) {
		return duplicate(line, row, importPlaceDuplicate+errorParam+name, process)
	}
	seen[key] = true
	if process {
		dto := &models.CreateStopPlaceDto{Name: name, Address: optionalCell(row, "address"), Latitude: lat, Longitude: lng}
		if _, err := d.svc.CreateStopPlace(ctx.Context(), ctx.TenantID, dto); err != nil {
			return rowResult(line, row, []string{importCreateFailed}, true)
		}
	}
	return rowResult(line, row, nil, process)
}

func (d *stopPlacesImport) nameTaken(ctx importModels.ImportContext, name string) bool {
	res, err := d.svc.ListStopPlaces(ctx.Context(), ctx.TenantID, models.ListStopPlacesQuery{Search: name, Page: 1, PageSize: 20}, nil, nil)
	if err != nil {
		return false
	}
	for _, p := range res.StopPlaces {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

// ── riders ───────────────────────────────────────────────────────────────

type ridersImport struct {
	svc trackingInterfaces.TrackingService
}

// NewRidersImportDescriptor imports the people a tenant carries, with their home and guardian contact.
func NewRidersImportDescriptor(svc trackingInterfaces.TrackingService) importInterfaces.ImportDescriptor {
	return &ridersImport{svc: svc}
}

func (d *ridersImport) Resource() string { return "tracking_riders" }

func (d *ridersImport) Columns() []importModels.ColumnSpec {
	return columnsOf(textColumn("first_name", true), textColumn("last_name", true), textColumn("organization", false),
		textColumn("phone", false), textColumn("address", false), numberColumn("home_latitude", false),
		numberColumn("home_longitude", false), textColumn("guardian_name", false), textColumn("guardian_phone", false),
		textColumn("guardian_email", false))
}

// Matches by the rider's own columns: the users file has first and last names but no organization
// nor home.
func (d *ridersImport) Matches(headers []string) bool {
	return hasAll(headers, "first_name", "last_name") &&
		(hasAll(headers, "organization") || hasAll(headers, "home_latitude") || hasAll(headers, "guardian_name"))
}

func (d *ridersImport) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, false)
}

func (d *ridersImport) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, true)
}

func (d *ridersImport) RecordRun(importModels.ImportContext, importModels.RunMeta) {}

func (d *ridersImport) run(ctx importModels.ImportContext, line int, row map[string]string, process bool) importModels.RowResult {
	dto, errs := d.dto(ctx, row)
	if len(errs) > 0 {
		return rowResult(line, row, errs, process)
	}
	key := dto.OrganizationID.String() + "|" + strings.ToLower(dto.FirstName+" "+dto.LastName)
	seen := scratchSet(ctx, "tracking.riders")
	if seen[key] || d.riderExists(ctx, dto) {
		return duplicate(line, row, importRiderDuplicate+errorParam+dto.FirstName+" "+dto.LastName, process)
	}
	seen[key] = true
	if process {
		if _, err := d.svc.CreateRider(ctx.Context(), ctx.TenantID, dto, nil); err != nil {
			return rowResult(line, row, []string{importCreateFailed}, true)
		}
	}
	return rowResult(line, row, nil, process)
}

func (d *ridersImport) dto(ctx importModels.ImportContext, row map[string]string) (*models.CreateRiderDto, []string) {
	var errs []string
	first, last := cell(row, "first_name"), cell(row, "last_name")
	if len(first) < 2 {
		errs = append(errs, importRequired+errorParam+"first_name")
	}
	if len(last) < 2 {
		errs = append(errs, importRequired+errorParam+"last_name")
	}
	orgs, kinds := organizationsOf(ctx, d.svc)
	orgID, orgErr := orgs.resolve(cell(row, "organization"), importOrgUnknown, importOrgRequired)
	if orgErr != "" {
		errs = append(errs, orgErr)
	}
	lat, okLat := parseNumber(cell(row, "home_latitude"), 64)
	lng, okLng := parseNumber(cell(row, "home_longitude"), 64)
	if !okLat || !okLng || (lat == nil) != (lng == nil) {
		errs = append(errs, importCoordinatesInvalid)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	riderType := "student"
	if kinds[orgID] == "company" {
		riderType = "employee"
	}
	return &models.CreateRiderDto{
		OrganizationID: orgID, RiderType: riderType, FirstName: first, LastName: last,
		Phone: optionalCell(row, "phone"), Address: optionalCell(row, "address"),
		HomeLatitude: lat, HomeLongitude: lng,
		GuardianName: optionalCell(row, "guardian_name"), GuardianPhone: optionalCell(row, "guardian_phone"),
		GuardianEmail: optionalCell(row, "guardian_email"),
	}, nil
}

func (d *ridersImport) riderExists(ctx importModels.ImportContext, dto *models.CreateRiderDto) bool {
	org := dto.OrganizationID.String()
	res, err := d.svc.ListRiders(ctx.Context(), ctx.TenantID, models.ListRidersQuery{
		OrganizationID: &org, Search: dto.LastName, Page: 1, PageSize: 100,
	}, nil, nil)
	if err != nil {
		return false
	}
	for _, r := range res.Riders {
		if strings.EqualFold(r.FirstName, dto.FirstName) && strings.EqualFold(r.LastName, dto.LastName) {
			return true
		}
	}
	return false
}

// ── document types ───────────────────────────────────────────────────────

type documentTypesImport struct {
	svc trackingInterfaces.TrackingService
}

// NewDocumentTypesImportDescriptor imports the tenant's compliance policy: which documents it tracks.
func NewDocumentTypesImportDescriptor(svc trackingInterfaces.TrackingService) importInterfaces.ImportDescriptor {
	return &documentTypesImport{svc: svc}
}

func (d *documentTypesImport) Resource() string { return "tracking_document_types" }

func (d *documentTypesImport) Columns() []importModels.ColumnSpec {
	return columnsOf(textColumn("code", true), textColumn("document", true), textColumn("applies_to", true),
		numberColumn("warn_days_before", false), textColumn("blocks_service", false))
}

func (d *documentTypesImport) Matches(headers []string) bool {
	return hasAll(headers, "code", "document", "applies_to")
}

func (d *documentTypesImport) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, false)
}

func (d *documentTypesImport) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, true)
}

func (d *documentTypesImport) RecordRun(importModels.ImportContext, importModels.RunMeta) {}

func (d *documentTypesImport) run(ctx importModels.ImportContext, line int, row map[string]string, process bool) importModels.RowResult {
	dto, errs := documentTypeDto(row)
	if len(errs) > 0 {
		return rowResult(line, row, errs, process)
	}
	seen := scratchSet(ctx, "tracking.document-codes")
	if seen[dto.Code] || d.codeTaken(ctx, dto.Code) {
		return duplicate(line, row, importCodeDuplicate+errorParam+dto.Code, process)
	}
	seen[dto.Code] = true
	if process {
		if _, err := d.svc.CreateDocumentType(ctx.Context(), ctx.TenantID, dto); err != nil {
			return rowResult(line, row, []string{importCreateFailed}, true)
		}
	}
	return rowResult(line, row, nil, process)
}

// documentTypeDto reads one policy line; "yes", "si", "true" or "1" block service.
func documentTypeDto(row map[string]string) (*models.CreateDocumentTypeDto, []string) {
	dto := &models.CreateDocumentTypeDto{
		Code: strings.ToUpper(cell(row, "code")), Name: cell(row, "document"), AppliesTo: strings.ToLower(cell(row, "applies_to")),
	}
	var errs []string
	if len(dto.Code) < 2 {
		errs = append(errs, importRequired+errorParam+"code")
	}
	if len(dto.Name) < 2 {
		errs = append(errs, importRequired+errorParam+"document")
	}
	if !appliesTo[dto.AppliesTo] {
		errs = append(errs, importTypeInvalid+errorParam+dto.AppliesTo)
	}
	warn, okWarn := parseNumber(cell(row, "warn_days_before"), 32)
	if !okWarn || (warn != nil && (*warn < 0 || *warn > 365)) {
		errs = append(errs, importNumberInvalid+errorParam+"warn_days_before")
	}
	if warn != nil {
		w := int32(*warn)
		dto.WarnDaysBefore = &w
	}
	dto.BlocksService = yesNo(cell(row, "blocks_service"))
	return dto, errs
}

var (
	appliesTo = map[string]bool{"vehicle": true, "driver": true, "both": true}
	yesWords  = map[string]bool{"yes": true, "si": true, "sí": true, "true": true, "1": true}
)

// yesNo reads a yes/no cell in either language; nil when empty.
func yesNo(value string) *bool {
	if value == "" {
		return nil
	}
	yes := yesWords[strings.ToLower(value)]
	return &yes
}

func (d *documentTypesImport) codeTaken(ctx importModels.ImportContext, code string) bool {
	types, err := d.svc.ListDocumentTypes(ctx.Context(), ctx.TenantID, models.ListDocumentTypesQuery{})
	if err != nil {
		return false
	}
	for _, t := range types {
		if strings.EqualFold(t.Code, code) {
			return true
		}
	}
	return false
}
