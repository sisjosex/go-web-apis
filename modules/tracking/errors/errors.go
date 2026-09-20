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
	EventInvalidType  = "tracking.event.invalid-type"

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
	VehicleListFailed     = "tracking.vehicle.list.failed"
	RouteListFailed       = "tracking.route.list.failed"
	RouteStopNotFound     = "tracking.route-stop.not-found"
	RouteStopCreateFailed = "tracking.route-stop.create.failed"
	RouteStopListFailed   = "tracking.route-stop.list.failed"
	RouteStopDeleteFailed = "tracking.route-stop.delete.failed"
	RiderListFailed       = "tracking.rider.list.failed"
	AssignmentListFailed  = "tracking.assignment.list.failed"

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
