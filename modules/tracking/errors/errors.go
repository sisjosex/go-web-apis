package errors

// Tracking module error codes
const (
	// Vehicle errors (TR0001-TR0020)
	VehicleNotFound           = "tracking.vehicle.not-found"
	VehicleAlreadyExists      = "tracking.vehicle.already-exists"
	VehiclePlateAlreadyExists = "tracking.vehicle.plate-already-exists"
	VehicleInvalidType        = "tracking.vehicle.invalid-type"
	VehicleCreateFailed       = "tracking.vehicle.create.failed"
	VehicleUpdateFailed       = "tracking.vehicle.update.failed"
	VehicleDeleteFailed       = "tracking.vehicle.delete.failed"
	VehicleInvalidLicense     = "tracking.vehicle.invalid-license"

	// Route errors (TR0021-TR0040)
	RouteNotFound            = "tracking.route.not-found"
	RouteAlreadyExists       = "tracking.route.already-exists"
	RouteCreateFailed        = "tracking.route.create.failed"
	RouteUpdateFailed        = "tracking.route.update.failed"
	RouteDeleteFailed        = "tracking.route.delete.failed"
	RouteInvalidScheduleType = "tracking.route.invalid-schedule-type"
	RouteCodeAlreadyExists   = "tracking.route.code-already-exists"
	RouteTimezone            = "tracking.route.timezone"

	// Rider errors (TR0041-TR0060)
	RiderNotFound      = "tracking.rider.not-found"
	RiderAlreadyExists = "tracking.rider.already-exists"
	RiderCreateFailed  = "tracking.rider.create.failed"
	RiderUpdateFailed  = "tracking.rider.update.failed"
	RiderDeleteFailed  = "tracking.rider.delete.failed"

	// Assignment errors (TR0061-TR0080)
	AssignmentNotFound      = "tracking.assignment.not-found"
	AssignmentCreateFailed  = "tracking.assignment.create.failed"
	AssignmentDeleteFailed  = "tracking.assignment.delete.failed"
	AssignmentAlreadyExists = "tracking.assignment.already-exists"

	// Location errors (TR0081-TR0100)
	LocationUpdateFailed  = "tracking.location.update.failed"
	LocationInvalidCoords = "tracking.location.invalid-coordinates"
	LocationNotFound      = "tracking.location.not-found"

	// Event errors (TR0101-TR0120)
	EventRecordFailed = "tracking.event.record.failed"
	EventNotFound     = "tracking.event.not-found"

	// Alert errors (TR0121-TR0140)
	AlertCreateFailed  = "tracking.alert.create.failed"
	AlertNotFound      = "tracking.alert.not-found"
	AlertUpdateFailed  = "tracking.alert.update.failed"
	AlertResolveFailed = "tracking.alert.resolve.failed"

	// Company errors (TR0141-TR0160)
	CompanyNotFound      = "tracking.company.not-found"
	CompanyCreateFailed  = "tracking.company.create.failed"
	CompanyUpdateFailed  = "tracking.company.update.failed"
	CompanyDeleteFailed  = "tracking.company.delete.failed"
	CompanyAlreadyExists = "tracking.company.already-exists"
	CompanyListFailed    = "tracking.company.list.failed"
	CompanyHasVehicles   = "tracking.company.has-vehicles"

	// Additional resource list errors
	VehicleListFailed    = "tracking.vehicle.list.failed"
	RouteListFailed      = "tracking.route.list.failed"
	RouteStopListFailed  = "tracking.route-stop.list.failed"
	RiderListFailed      = "tracking.rider.list.failed"
	AssignmentListFailed = "tracking.assignment.list.failed"

	// Organization errors (TR0161-TR0180)
	OrganizationNotFound     = "tracking.organization.not-found"
	OrganizationCreateFailed = "tracking.organization.create.failed"
	OrganizationUpdateFailed = "tracking.organization.update.failed"
	OrganizationDeleteFailed = "tracking.organization.delete.failed"
	OrganizationListFailed   = "tracking.organization.list.failed"
	OrganizationHasRiders    = "tracking.organization.has-riders"
	// OrganizationScopeDenied is raised by fn_organization_scope, verbatim: an organization-level
	// user who belongs to no organization in this tenant (TRACK-015 D1).
	OrganizationScopeDenied = "tracking.organization.scope-denied"

	// Organization member errors
	OrganizationMemberNotFound     = "tracking.organization-member.not-found"
	OrganizationMemberUserNotFound = "tracking.organization-member.user-not-found"
	OrganizationMemberSaveFailed   = "tracking.organization-member.save.failed"
	OrganizationMemberListFailed   = "tracking.organization-member.list.failed"

	// Route version errors (TR0241-TR0260)
	RouteVersionNotFound     = "tracking.route.version-not-found"
	RouteVersionCreateFailed = "tracking.route.version-create.failed"
	RouteVersionListFailed   = "tracking.route.version-list.failed"
	// RouteVersionOverlap refuses a version that would not start after the latest one: history is
	// appended to, never spliced.
	RouteVersionOverlap = "tracking.route.version-overlap"
	// RouteVersionClosed refuses editing the stop list of a version whose first day has passed —
	// that list is the record of what the route ran.
	RouteVersionClosed    = "tracking.route.version-closed"
	RouteVersionSaveStops = "tracking.route.version-stops.failed"

	// Route schedule errors (TR0261-TR0280)
	ScheduleNotFound     = "tracking.schedule.not-found"
	ScheduleCreateFailed = "tracking.schedule.create.failed"
	ScheduleUpdateFailed = "tracking.schedule.update.failed"
	ScheduleDeleteFailed = "tracking.schedule.delete.failed"
	ScheduleListFailed   = "tracking.schedule.list.failed"
	ScheduleSplitFailed  = "tracking.schedule.split.failed"
	// ScheduleOverlap refuses a schedule that would leave the route with two departures at the same
	// time of day on the same date — the exclusion constraint is the backstop for two writers racing.
	ScheduleOverlap = "tracking.schedule.overlap"
	// ScheduleSplitDate refuses a split outside the schedule's own validity: before it starts there is
	// nothing to close, after it ends there is nothing to carry.
	ScheduleSplitDate = "tracking.schedule.split-date"
	// ScheduleDaysOfWeek refuses a weekday bitmask outside 1..127 arriving through a split's changes,
	// which the DTO cannot reach.
	ScheduleDaysOfWeek = "tracking.schedule.days-of-week"

	// Calendar errors (TR0281-TR0300)
	CalendarNotFound     = "tracking.calendar.not-found"
	CalendarCreateFailed = "tracking.calendar.create.failed"
	CalendarUpdateFailed = "tracking.calendar.update.failed"
	CalendarDeleteFailed = "tracking.calendar.delete.failed"
	CalendarListFailed   = "tracking.calendar.list.failed"
	CalendarDatesFailed  = "tracking.calendar.dates.failed"
	// CalendarInUse refuses a delete while a schedule still points at the calendar: the schedule would
	// silently start running on the holidays.
	CalendarInUse = "tracking.calendar.in-use"
	// CalendarDuplicateDate refuses a replace whose list names one date twice — which of the two the
	// calendar would keep is not the caller's to leave unsaid.
	CalendarDuplicateDate = "tracking.calendar.duplicate-date"

	// Route exception errors (TR0301-TR0320)
	ExceptionNotFound     = "tracking.exception.not-found"
	ExceptionCreateFailed = "tracking.exception.create.failed"
	ExceptionDeleteFailed = "tracking.exception.delete.failed"
	ExceptionListFailed   = "tracking.exception.list.failed"
	// ExceptionPayload refuses a payload that does not carry exactly the keys its kind defines: an
	// extra key is a caller saying something the SP will not read.
	ExceptionPayload = "tracking.exception.payload"

	// Preview errors
	RoutePreviewFailed = "tracking.route.preview.failed"
	// RoutePreviewRange refuses a window wider than 92 days — a quarter is what the screen shows.
	RoutePreviewRange = "tracking.route.preview-range"

	// Stop place errors (TR0221-TR0240)
	StopPlaceNotFound     = "tracking.stop-place.not-found"
	StopPlaceCreateFailed = "tracking.stop-place.create.failed"
	StopPlaceUpdateFailed = "tracking.stop-place.update.failed"
	StopPlaceDeleteFailed = "tracking.stop-place.delete.failed"
	StopPlaceListFailed   = "tracking.stop-place.list.failed"
	// StopPlaceInUse refuses a delete while a route version still names the stop place: the version
	// is the record of what a route ran, and removing a stop from it rewrites history.
	StopPlaceInUse = "tracking.stop-place.in-use"

	// Driver errors (TR0181-TR0200)
	DriverNotFound             = "tracking.driver.not-found"
	DriverCreateFailed         = "tracking.driver.create.failed"
	DriverUpdateFailed         = "tracking.driver.update.failed"
	DriverDeleteFailed         = "tracking.driver.delete.failed"
	DriverListFailed           = "tracking.driver.list.failed"
	DriverLicenseAlreadyExists = "tracking.driver.license-already-exists"
	DriverUserAlreadyLinked    = "tracking.driver.user-already-linked"
	// DriverHasRoutes refuses a delete while a route still names the driver as its default.
	DriverHasRoutes = "tracking.driver.has-routes"

	// Document errors (TR0201-TR0220)
	DocumentNotFound          = "tracking.document.not-found"
	DocumentCreateFailed      = "tracking.document.create.failed"
	DocumentUpdateFailed      = "tracking.document.update.failed"
	DocumentDeleteFailed      = "tracking.document.delete.failed"
	DocumentListFailed        = "tracking.document.list.failed"
	DocumentSubjectNotFound   = "tracking.document.subject-not-found"
	DocumentTypeNotApplicable = "tracking.document.type-not-applicable"
	// DocumentFileInvalid covers every way an upload is refused: the wrong extension, content that
	// is not what the extension claims, or more bytes than the cap (TRACK-016 D3).
	DocumentFileInvalid  = "tracking.document.file-invalid"
	DocumentFileNotFound = "tracking.document.file-not-found"
	DocumentFileFailed   = "tracking.document.file.failed"

	DocumentTypeNotFound          = "tracking.document-type.not-found"
	DocumentTypeCreateFailed      = "tracking.document-type.create.failed"
	DocumentTypeUpdateFailed      = "tracking.document-type.update.failed"
	DocumentTypeListFailed        = "tracking.document-type.list.failed"
	DocumentTypeCodeAlreadyExists = "tracking.document-type.code-already-exists"

	TenantNotFound = "tracking.tenant.not-found"

	// Generic errors
	TrackingUnauthorized  = "tracking.unauthorized"
	TrackingForbidden     = "tracking.forbidden"
	TrackingInternalError = "tracking.internal-error"
	TenantRequired        = "tracking.tenant.required"
)

// TrackingError represents a tracking module error
type TrackingError struct {
	Code string
	Err  error
}

func (e *TrackingError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

// Unwrap returns the wrapped error, allowing errors.As to work
func (e *TrackingError) Unwrap() error {
	return e.Err
}
