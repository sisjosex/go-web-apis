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

	// Organization member errors
	OrganizationMemberNotFound     = "tracking.organization-member.not-found"
	OrganizationMemberUserNotFound = "tracking.organization-member.user-not-found"
	OrganizationMemberSaveFailed   = "tracking.organization-member.save.failed"
	OrganizationMemberListFailed   = "tracking.organization-member.list.failed"

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
