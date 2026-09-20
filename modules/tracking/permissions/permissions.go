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
)

func init() {
	permRegistry.Register([]models.AvailablePermission{
		{Code: CompaniesRead, Module: "tracking", Name: "permission.tracking.companies-read", Description: "permission.tracking.companies-read.description"},
		{Code: CompaniesWrite, Module: "tracking", Name: "permission.tracking.companies-write", Description: "permission.tracking.companies-write.description"},
		{Code: CompaniesDelete, Module: "tracking", Name: "permission.tracking.companies-delete", Description: "permission.tracking.companies-delete.description"},
		{Code: VehiclesRead, Module: "tracking", Name: "permission.tracking.vehicles-read", Description: "permission.tracking.vehicles-read.description"},
		{Code: VehiclesWrite, Module: "tracking", Name: "permission.tracking.vehicles-write", Description: "permission.tracking.vehicles-write.description"},
		{Code: VehiclesDelete, Module: "tracking", Name: "permission.tracking.vehicles-delete", Description: "permission.tracking.vehicles-delete.description"},
		{Code: RoutesRead, Module: "tracking", Name: "permission.tracking.routes-read", Description: "permission.tracking.routes-read.description"},
		{Code: RoutesWrite, Module: "tracking", Name: "permission.tracking.routes-write", Description: "permission.tracking.routes-write.description"},
		{Code: RoutesDelete, Module: "tracking", Name: "permission.tracking.routes-delete", Description: "permission.tracking.routes-delete.description"},
		{Code: RidersRead, Module: "tracking", Name: "permission.tracking.riders-read", Description: "permission.tracking.riders-read.description"},
		{Code: RidersWrite, Module: "tracking", Name: "permission.tracking.riders-write", Description: "permission.tracking.riders-write.description"},
		{Code: RidersDelete, Module: "tracking", Name: "permission.tracking.riders-delete", Description: "permission.tracking.riders-delete.description"},
		{Code: AssignmentsRead, Module: "tracking", Name: "permission.tracking.assignments-read", Description: "permission.tracking.assignments-read.description"},
		{Code: AssignmentsManage, Module: "tracking", Name: "permission.tracking.assignments-manage", Description: "permission.tracking.assignments-manage.description"},
		{Code: EventsWrite, Module: "tracking", Name: "permission.tracking.events-write", Description: "permission.tracking.events-write.description"},
		{Code: AlertsWrite, Module: "tracking", Name: "permission.tracking.alerts-write", Description: "permission.tracking.alerts-write.description"},
		{Code: OrganizationsRead, Module: "tracking", Name: "permission.tracking.organizations-read", Description: "permission.tracking.organizations-read.description"},
		{Code: OrganizationsWrite, Module: "tracking", Name: "permission.tracking.organizations-write", Description: "permission.tracking.organizations-write.description"},
		{Code: OrganizationsDelete, Module: "tracking", Name: "permission.tracking.organizations-delete", Description: "permission.tracking.organizations-delete.description"},
	})
}
