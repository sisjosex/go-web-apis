// Package permissions defines all permission codes for the tracking module
// and registers them in the global tenancy permission registry.
package permissions

import (
	"josex/web/modules/tenancy/models"
	permRegistry "josex/web/modules/tenancy/permissions"
)

// Permission code constants — use these in RequirePermission() calls.
const (
	CompaniesRead   = "tracking:companies:read"
	CompaniesWrite  = "tracking:companies:write"
	CompaniesDelete = "tracking:companies:delete"

	VehiclesRead   = "tracking:vehicles:read"
	VehiclesWrite  = "tracking:vehicles:write"
	VehiclesDelete = "tracking:vehicles:delete"

	RoutesRead   = "tracking:routes:read"
	RoutesWrite  = "tracking:routes:write"
	RoutesDelete = "tracking:routes:delete"

	RidersRead   = "tracking:riders:read"
	RidersWrite  = "tracking:riders:write"
	RidersDelete = "tracking:riders:delete"

	AssignmentsRead   = "tracking:assignments:read"
	AssignmentsManage = "tracking:assignments:manage"

	EventsWrite = "tracking:events:write"
	AlertsWrite = "tracking:alerts:write"

	OrganizationsRead   = "tracking:organizations:read"
	OrganizationsWrite  = "tracking:organizations:write"
	OrganizationsDelete = "tracking:organizations:delete"

	DriversWrite  = "tracking:drivers:write"
	DriversDelete = "tracking:drivers:delete"

	// DocumentsWrite covers deleting one too (TRACK-016): filing a document and withdrawing it are
	// the same job, and a separate delete permission no role would hold alone is noise.
	DocumentsWrite = "tracking:documents:write"

	// ReportsRead opens the reports (TRACK-013 D4): a role can see attendance and punctuality without
	// seeing routes, stops or the live fleet.
	ReportsRead = "tracking:reports:read"
)

func init() {
	permRegistry.RegisterSections("tracking", "live", "trips", "clients_riders", "drivers_vehicles", "routes_stops", "documents")
	permRegistry.Register([]models.AvailablePermission{
		{Code: CompaniesRead, Module: "tracking", Name: "permission.tracking.companies-read", Description: "permission.tracking.companies-read.description", Section: "drivers_vehicles", Access: models.AccessRead},
		{Code: CompaniesWrite, Module: "tracking", Name: "permission.tracking.companies-write", Description: "permission.tracking.companies-write.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: CompaniesDelete, Module: "tracking", Name: "permission.tracking.companies-delete", Description: "permission.tracking.companies-delete.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: VehiclesRead, Module: "tracking", Name: "permission.tracking.vehicles-read", Description: "permission.tracking.vehicles-read.description", Section: "drivers_vehicles", Access: models.AccessRead, AlsoIn: []string{"documents"}},
		{Code: VehiclesWrite, Module: "tracking", Name: "permission.tracking.vehicles-write", Description: "permission.tracking.vehicles-write.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: VehiclesDelete, Module: "tracking", Name: "permission.tracking.vehicles-delete", Description: "permission.tracking.vehicles-delete.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: RoutesRead, Module: "tracking", Name: "permission.tracking.routes-read", Description: "permission.tracking.routes-read.description", Section: "routes_stops", Access: models.AccessRead, AlsoIn: []string{"live", "trips"}},
		{Code: RoutesWrite, Module: "tracking", Name: "permission.tracking.routes-write", Description: "permission.tracking.routes-write.description", Section: "routes_stops", Access: models.AccessWrite},
		{Code: RoutesDelete, Module: "tracking", Name: "permission.tracking.routes-delete", Description: "permission.tracking.routes-delete.description", Section: "routes_stops", Access: models.AccessWrite},
		{Code: RidersRead, Module: "tracking", Name: "permission.tracking.riders-read", Description: "permission.tracking.riders-read.description", Section: "clients_riders", Access: models.AccessRead},
		{Code: RidersWrite, Module: "tracking", Name: "permission.tracking.riders-write", Description: "permission.tracking.riders-write.description", Section: "clients_riders", Access: models.AccessWrite},
		{Code: RidersDelete, Module: "tracking", Name: "permission.tracking.riders-delete", Description: "permission.tracking.riders-delete.description", Section: "clients_riders", Access: models.AccessWrite},
		{Code: AssignmentsRead, Module: "tracking", Name: "permission.tracking.assignments-read", Description: "permission.tracking.assignments-read.description", Section: "clients_riders", Access: models.AccessRead},
		{Code: AssignmentsManage, Module: "tracking", Name: "permission.tracking.assignments-manage", Description: "permission.tracking.assignments-manage.description", Section: "clients_riders", Access: models.AccessWrite},
		{Code: EventsWrite, Module: "tracking", Name: "permission.tracking.events-write", Description: "permission.tracking.events-write.description", Section: "trips", Access: models.AccessWrite},
		{Code: AlertsWrite, Module: "tracking", Name: "permission.tracking.alerts-write", Description: "permission.tracking.alerts-write.description", Section: "live", Access: models.AccessWrite},
		{Code: OrganizationsRead, Module: "tracking", Name: "permission.tracking.organizations-read", Description: "permission.tracking.organizations-read.description", Section: "clients_riders", Access: models.AccessRead},
		{Code: OrganizationsWrite, Module: "tracking", Name: "permission.tracking.organizations-write", Description: "permission.tracking.organizations-write.description", Section: "clients_riders", Access: models.AccessWrite},
		{Code: OrganizationsDelete, Module: "tracking", Name: "permission.tracking.organizations-delete", Description: "permission.tracking.organizations-delete.description", Section: "clients_riders", Access: models.AccessWrite},
		{Code: DriversWrite, Module: "tracking", Name: "permission.tracking.drivers-write", Description: "permission.tracking.drivers-write.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: DriversDelete, Module: "tracking", Name: "permission.tracking.drivers-delete", Description: "permission.tracking.drivers-delete.description", Section: "drivers_vehicles", Access: models.AccessWrite},
		{Code: DocumentsWrite, Module: "tracking", Name: "permission.tracking.documents-write", Description: "permission.tracking.documents-write.description", Section: "documents", Access: models.AccessWrite},
		{Code: ReportsRead, Module: "tracking", Name: "permission.tracking.reports-read", Description: "permission.tracking.reports-read.description", Section: "trips", Access: models.AccessRead},
	})
}
