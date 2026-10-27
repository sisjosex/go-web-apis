package services

import (
	"strings"
	"time"

	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	trackingInterfaces "josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The routes import (TRACK-075): each row is a client's route as the route form creates it — an
// ingreso to the client's pin with its arrival time, or the retorno of the ingreso of the same name,
// leaving at its departure time. Stops are drawn in the route's detail, not imported.

const (
	importPlateUnknown     = "tracking.import.plate-unknown"
	importDriverUnknown    = "tracking.import.driver-unknown"
	importCompanyMismatch  = "tracking.import.company-mismatch"
	importOutboundUnknown  = "tracking.import.outbound-unknown"
	importReturnDuplicate  = "tracking.import.return-duplicate"
	importTimeInvalid      = "tracking.import.time-invalid"
	importDaysInvalid      = "tracking.import.days-invalid"
	importOrgWithoutPin    = "tracking.import.organization-without-location"
	importDirectionInvalid = "tracking.import.direction-invalid"

	// weekdays is Monday to Friday, the days a row without "días" runs (the route form's default).
	weekdays int16 = 31
)

type routesImport struct {
	svc trackingInterfaces.TrackingService
}

// NewRoutesImportDescriptor imports a carrier's routes, each with its schedule.
func NewRoutesImportDescriptor(svc trackingInterfaces.TrackingService) importInterfaces.ImportDescriptor {
	return &routesImport{svc: svc}
}

func (d *routesImport) Resource() string { return "tracking_routes" }

func (d *routesImport) Columns() []importModels.ColumnSpec {
	return columnsOf(textColumn("route_name", true, "nombre", "nombre de la ruta", "ruta", "name", "route"),
		textColumn("route_code", false, "codigo", "codigo de ruta", "code"),
		textColumn("direction", false, "sentido", "ingreso o retorno"),
		textColumn("organization", false, "cliente", "colegio", "client"),
		textColumn("company", false, "empresa", "transportista", "carrier"),
		textColumn("plate", false, "placa", "vehiculo", "bus", "vehicle"),
		textColumn("driver", false, "conductor", "chofer"),
		textColumn("time", false, "hora", "hora de llegada", "hora de salida", "llegada", "salida"),
		textColumn("days", false, "dias", "dias de servicio"))
}

func (d *routesImport) Matches(headers []string) bool {
	return hasAll(headers, "route_name", "time") || hasAll(headers, "route_name", "direction")
}

func (d *routesImport) KeyColumns() []string { return []string{"route_code"} }

func (d *routesImport) ValidateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, false)
}

func (d *routesImport) ProcessRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	return d.run(ctx, line, row, true)
}

func (d *routesImport) RecordRun(importModels.ImportContext, importModels.RunMeta) {}

// routeRow is a row read and resolved against the tenant.
type routeRow struct {
	name      string
	code      *string
	inbound   bool
	org       *models.Organization
	companyID uuid.UUID
	vehicleID *uuid.UUID
	driverID  *uuid.UUID
	time      *string
	days      int16
}

// UpdateRow writes a known code's name, vehicle and driver over the route (IMPORT-002 D4); its
// schedule stays, a time change being a split of its own.
func (d *routesImport) UpdateRow(ctx importModels.ImportContext, line int, row map[string]string) importModels.RowResult {
	r, errs := d.read(ctx, row)
	if len(errs) > 0 {
		return rowResult(line, row, errs, true)
	}
	var existing *models.Route
	if r.code != nil {
		existing = d.findByCode(ctx, *r.code)
	}
	if existing == nil {
		return importModels.RowResult{Line: line, Data: row, Status: importModels.RowStatusSkipped}
	}
	update := &models.UpdateRouteDto{RouteName: &r.name, VehicleID: r.vehicleID, DefaultDriverID: r.driverID}
	if _, err := d.svc.UpdateRoute(ctx.Context(), ctx.TenantID, existing.ID, update); err != nil {
		return rowResult(line, row, []string{importCreateFailed}, true)
	}
	return updated(line, row)
}

func (d *routesImport) run(ctx importModels.ImportContext, line int, row map[string]string, process bool) importModels.RowResult {
	r, errs := d.read(ctx, row)
	if len(errs) > 0 {
		return rowResult(line, row, errs, process)
	}
	if r.code != nil {
		codes := scratchSet(ctx, "tracking.route-codes")
		if codes[*r.code] || d.findByCode(ctx, *r.code) != nil {
			return duplicate(line, row, importCodeDuplicate+errorParam+*r.code, process)
		}
		codes[*r.code] = true
	}
	// An ingreso earlier in the file is the outbound a retorno below it pairs with, before either exists.
	key := r.org.ID.String() + "|" + strings.ToLower(r.name)
	outbounds := scratchSet(ctx, "tracking.route-outbounds")
	if !r.inbound {
		outbounds[key] = true
		if process {
			return d.createOutbound(ctx, line, row, r)
		}
		return rowResult(line, row, nil, false)
	}
	returns := scratchSet(ctx, "tracking.route-returns")
	outbound := d.findOutbound(ctx, r)
	if returns[key] || (outbound != nil && outbound.PairedRouteID != nil) {
		return duplicate(line, row, importReturnDuplicate+errorParam+r.name, process)
	}
	if outbound == nil && !outbounds[key] {
		return rowResult(line, row, []string{importOutboundUnknown + errorParam + r.name}, process)
	}
	returns[key] = true
	if !process {
		return rowResult(line, row, nil, false)
	}
	if outbound == nil {
		return rowResult(line, row, []string{importOutboundUnknown + errorParam + r.name}, true)
	}
	created, err := d.svc.CreateRouteReturn(ctx.Context(), ctx.TenantID, outbound.ID, &models.CreateRouteReturnDto{DepartureTime: *r.time}, ctx.PerformedBy)
	if err != nil {
		return rowResult(line, row, []string{importCreateFailed}, true)
	}
	return d.finish(ctx, line, row, created.ID, r)
}

func (d *routesImport) createOutbound(ctx importModels.ImportContext, line int, row map[string]string, r *routeRow) importModels.RowResult {
	withReturn := false
	dto := &models.CreateRouteDto{
		CompanyID: r.companyID, RouteName: r.name, OrganizationID: &r.org.ID, WithReturn: &withReturn,
		VehicleID: r.vehicleID, ArrivalTime: r.time, DaysOfWeek: &r.days, DestinationStopPlaceID: r.org.StopPlaceID,
	}
	if r.org.StopPlaceID == nil {
		dto.DestinationLat, dto.DestinationLng = r.org.Latitude, r.org.Longitude
	}
	created, err := d.svc.CreateRoute(ctx.Context(), ctx.TenantID, dto, ctx.PerformedBy)
	if err != nil {
		return rowResult(line, row, []string{importCreateFailed}, true)
	}
	return d.finish(ctx, line, row, created.ID, r)
}

// finish gives a created route the code and driver the create SPs do not take.
func (d *routesImport) finish(ctx importModels.ImportContext, line int, row map[string]string, id uuid.UUID, r *routeRow) importModels.RowResult {
	if r.code != nil || r.driverID != nil || (r.inbound && r.vehicleID != nil) {
		update := &models.UpdateRouteDto{RouteCode: r.code, DefaultDriverID: r.driverID}
		if r.inbound {
			update.VehicleID = r.vehicleID
		}
		if _, err := d.svc.UpdateRoute(ctx.Context(), ctx.TenantID, id, update); err != nil {
			return rowResult(line, row, []string{importCreateFailed}, true)
		}
	}
	return rowResult(line, row, nil, true)
}

func (d *routesImport) read(ctx importModels.ImportContext, row map[string]string) (*routeRow, []string) {
	var errs []string
	r := &routeRow{name: cell(row, "route_name"), days: weekdays}
	if len(r.name) < 3 {
		errs = append(errs, importRequired+errorParam+"route_name")
	}
	if code := strings.ToUpper(cell(row, "route_code")); code != "" {
		r.code = &code
	}
	inbound, ok := routeDirection(cell(row, "direction"))
	if !ok {
		errs = append(errs, importDirectionInvalid+errorParam+cell(row, "direction"))
	}
	r.inbound = inbound
	errs = append(errs, d.readParties(ctx, row, r)...)
	return r, append(errs, readSchedule(row, r)...)
}

// readParties resolves the row's client, carrier, vehicle and driver by name in the tenant.
func (d *routesImport) readParties(ctx importModels.ImportContext, row map[string]string, r *routeRow) []string {
	var errs []string
	orgs, byID := organizationsOf(ctx, d.svc)
	if orgID, orgErr := orgs.resolve(cell(row, "organization"), importOrgUnknown, importOrgRequired); orgErr != "" {
		errs = append(errs, orgErr)
	} else if r.org = byID[orgID]; r.org == nil {
		errs = append(errs, importOrgUnknown+errorParam+cell(row, "organization"))
	} else if r.org.StopPlaceID == nil && (r.org.Latitude == nil || r.org.Longitude == nil) {
		errs = append(errs, importOrgWithoutPin+errorParam+r.org.Name)
	}
	companyID, companyErr := companiesOf(ctx, d.svc).resolve(cell(row, "company"), importCompanyUnknown, importCompanyRequired)
	if companyErr != "" {
		return append(errs, companyErr)
	}
	r.companyID = companyID
	if plate := strings.ToUpper(cell(row, "plate")); plate != "" {
		v := d.findVehicle(ctx, plate)
		switch {
		case v == nil:
			errs = append(errs, importPlateUnknown+errorParam+plate)
		case v.CompanyID != companyID:
			errs = append(errs, importCompanyMismatch+errorParam+plate)
		default:
			r.vehicleID = &v.ID
		}
	}
	if name := cell(row, "driver"); name != "" {
		driver := d.findDriver(ctx, name)
		switch {
		case driver == nil:
			errs = append(errs, importDriverUnknown+errorParam+name)
		case driver.CompanyID != companyID:
			errs = append(errs, importCompanyMismatch+errorParam+name)
		default:
			r.driverID = &driver.ID
		}
	}
	return errs
}

// readSchedule reads the row's time, required on a retorno, and its days.
func readSchedule(row map[string]string, r *routeRow) []string {
	var errs []string
	if value := cell(row, "time"); value != "" {
		if hhmm, ok := clockTime(value); ok {
			r.time = &hhmm
		} else {
			errs = append(errs, importTimeInvalid+errorParam+value)
		}
	} else if r.inbound {
		errs = append(errs, importRequired+errorParam+"time")
	}
	if value := cell(row, "days"); value != "" {
		if mask, ok := weekdayMask(value); ok {
			r.days = mask
		} else {
			errs = append(errs, importDaysInvalid+errorParam+value)
		}
	}
	return errs
}

// routeDirection reads "ingreso / retorno" (or "ida / vuelta", "outbound / inbound"); empty is ingreso.
func routeDirection(value string) (bool, bool) {
	inbound, ok := map[string]bool{
		"": false, "ingreso": false, "ida": false, "outbound": false,
		"retorno": true, "vuelta": true, "salida": true, "inbound": true, "return": true,
	}[strings.ToLower(strings.TrimSpace(value))]
	return inbound, ok
}

// clockTime reads "7:30", "07:30" or Excel's "07:30:00" as HH:mm.
func clockTime(value string) (string, bool) {
	for _, layout := range []string{"15:04", "15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("15:04"), true
		}
	}
	return "", false
}

// weekdayMask reads "lun-vie", "lunes a viernes", "lun, mie, vie", "todos" (or the English names) as
// the weekday bitmask the schedules store, bit 0 Monday.
func weekdayMask(value string) (int16, bool) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "todos" || text == "diario" || text == "every day" || text == "daily" {
		return 127, true
	}
	text = strings.NewReplacer(" a ", "-", " to ", "-", "á", "a", "é", "e", "–", "-").Replace(text)
	var mask int16
	for _, part := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '/' || r == ';' }) {
		from, to, isRange := strings.Cut(part, "-")
		first, ok := weekdayIndex(from)
		if !ok {
			return 0, false
		}
		last := first
		if isRange {
			if last, ok = weekdayIndex(to); !ok || last < first {
				return 0, false
			}
		}
		for day := first; day <= last; day++ {
			mask |= 1 << day
		}
	}
	return mask, mask != 0
}

func weekdayIndex(name string) (int, bool) {
	prefixes := [7][]string{{"lu", "mo"}, {"ma", "tu"}, {"mi", "we"}, {"ju", "th"}, {"vi", "fr"}, {"sa"}, {"do", "su"}}
	if len(name) < 2 {
		return 0, false
	}
	for day, options := range prefixes {
		for _, prefix := range options {
			if strings.HasPrefix(name, prefix) {
				return day, true
			}
		}
	}
	return 0, false
}

// findByCode is the tenant's route with this code, or nil.
func (d *routesImport) findByCode(ctx importModels.ImportContext, code string) *models.Route {
	res, err := d.svc.ListRoutes(ctx.Context(), ctx.TenantID, models.ListRoutesQuery{Search: code, Page: 1, PageSize: 20}, nil)
	if err != nil {
		return nil
	}
	for _, route := range res.Routes {
		if route.RouteCode != nil && strings.EqualFold(*route.RouteCode, code) {
			return route
		}
	}
	return nil
}

// findOutbound is the client's ingreso with the row's name, or nil.
func (d *routesImport) findOutbound(ctx importModels.ImportContext, r *routeRow) *models.Route {
	orgID := r.org.ID.String()
	query := models.ListRoutesQuery{Search: r.name, OrganizationID: &orgID, Direction: "outbound", Page: 1, PageSize: 20}
	res, err := d.svc.ListRoutes(ctx.Context(), ctx.TenantID, query, nil)
	if err != nil {
		return nil
	}
	for _, route := range res.Routes {
		if strings.EqualFold(route.RouteName, r.name) {
			return route
		}
	}
	return nil
}

func (d *routesImport) findVehicle(ctx importModels.ImportContext, plate string) *models.Vehicle {
	res, err := d.svc.ListVehicles(ctx.Context(), ctx.TenantID, models.ListVehiclesQuery{Search: plate, Page: 1, PageSize: 20})
	if err != nil {
		return nil
	}
	for _, v := range res.Vehicles {
		if strings.EqualFold(v.PlateNumber, plate) {
			return v
		}
	}
	return nil
}

// findDriver is the tenant's driver whose full name is this one, searched by its last word.
func (d *routesImport) findDriver(ctx importModels.ImportContext, name string) *models.Driver {
	words := strings.Fields(name)
	res, err := d.svc.ListDrivers(ctx.Context(), ctx.TenantID, models.ListDriversQuery{Search: words[len(words)-1], Page: 1, PageSize: 50})
	if err != nil {
		return nil
	}
	for _, driver := range res.Drivers {
		if strings.EqualFold(strings.Join(strings.Fields(driver.FirstName+" "+driver.LastName), " "), strings.Join(words, " ")) {
			return driver
		}
	}
	return nil
}
