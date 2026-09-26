package routes

import (
	"josex/web/config"
	"josex/web/modules/auth/middleware"
	authServices "josex/web/modules/auth/services"
	tenancyMW "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"
	"josex/web/modules/tracking/controllers"
	trackingPerms "josex/web/modules/tracking/permissions"

	"github.com/gin-gonic/gin"
)

// RegisterTrackingRoutes registers all tracking module routes
// Routes work in both single-database and multi-tenant modes
// Controllers handle tenant validation based on config
// jwtService is required for authentication middleware on protected routes
// portalTenantMiddleware is the same chain with the mobile-only refusal lifted for a guardian
// (TRACK-017 D1). Only the two reads a guardian may make are registered behind it; everything else
// keeps tenantMiddleware and its blanket refusal.
func RegisterTrackingRoutes(
	router *gin.Engine,
	trackingController *controllers.TrackingController,
	tenantMiddleware gin.HandlerFunc,
	portalTenantMiddleware gin.HandlerFunc,
	jwtService authServices.JWTService,
) {
	trackingGroup := router.Group("/api/v1/tracking")

	// Check if multi-tenant mode is enabled
	coreConf := config.ModularAppConfig.Core
	tenancyConf := config.ModularAppConfig.Tenancy

	if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
		registerTenantRoutes(trackingGroup, trackingController, tenantMiddleware, portalTenantMiddleware, jwtService)
		return
	}
	registerOpenRoutes(trackingGroup, trackingController)
}

// registerTenantRoutes is the multi-tenant shape: one group per chain, then the areas of the
// module, each of which decides which access levels reach it.
func registerTenantRoutes(
	trackingGroup *gin.RouterGroup,
	trackingController *controllers.TrackingController,
	tenantMiddleware gin.HandlerFunc,
	portalTenantMiddleware gin.HandlerFunc,
	jwtService authServices.JWTService,
) {
	// Multi-tenant mode: auth + tenant middleware + per-route permission checks
	r := trackingGroup.Group("")
	r.Use(middleware.AuthMiddleware(jwtService))
	r.Use(tenantMiddleware)

	// An organization user sees its own people and the routes they ride, and edits those
	// riders — nothing else (TRACK-015 D1). The fleet, the organizations themselves and every
	// write outside riders are the operator's, so those routes deny the level outright rather
	// than relying on a scope the SP behind them does not take.
	denyOrganization := tenancyMW.DenyTenantRole(tenancyModels.RoleOrganization)

	// The guardian's group: auth, then the chain that lets a portal account through.
	p := trackingGroup.Group("")
	p.Use(middleware.AuthMiddleware(jwtService))
	p.Use(portalTenantMiddleware)

	registerFleetRoutes(r, denyOrganization, trackingController)
	registerRouteRoutes(r, denyOrganization, trackingController)
	registerStopPlaceRoutes(r, denyOrganization, trackingController)
	registerScheduleRoutes(r, denyOrganization, trackingController)
	registerCalendarRoutes(r, denyOrganization, trackingController)
	registerExceptionRoutes(r, denyOrganization, trackingController)
	registerOrganizationRoutes(r, denyOrganization, trackingController)
	registerDocumentRoutes(r, denyOrganization, trackingController)
	registerRiderRoutes(r, p, denyOrganization, trackingController)
	registerTripRoutes(r, denyOrganization, trackingController)
}

// registerFleetRoutes is what the operator owns and runs: its companies, its buses, its drivers.
func registerFleetRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Companies
	r.GET("/companies", denyOrganization, trackingController.ListCompanies)
	r.GET("/companies/:company_id", denyOrganization, trackingController.GetCompany)
	r.POST("/companies",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.CompaniesWrite),
		trackingController.CreateCompany)
	r.PATCH("/companies/:company_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.CompaniesWrite),
		trackingController.UpdateCompany)
	r.DELETE("/companies/:company_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.CompaniesDelete),
		trackingController.DeleteCompany)

	// Vehicles
	r.GET("/vehicles", denyOrganization, trackingController.ListVehicles)
	r.GET("/vehicles/:vehicle_id", denyOrganization, trackingController.GetVehicle)
	r.GET("/vehicles/:vehicle_id/location", denyOrganization, trackingController.GetVehicleCurrentLocation)
	r.POST("/vehicles",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.VehiclesWrite),
		trackingController.CreateVehicle)
	r.PATCH("/vehicles/:vehicle_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.VehiclesWrite),
		trackingController.UpdateVehicle)
	r.DELETE("/vehicles/:vehicle_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.VehiclesDelete),
		trackingController.DeleteVehicle)

	// Drivers — the carrier's staff, so the operator's alone, like the rest of the fleet.
	r.GET("/drivers", denyOrganization, trackingController.ListDrivers)
	r.GET("/drivers/:driver_id", denyOrganization, trackingController.GetDriver)
	r.POST("/drivers",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DriversWrite),
		trackingController.CreateDriver)
	r.PATCH("/drivers/:driver_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DriversWrite),
		trackingController.UpdateDriver)
	r.DELETE("/drivers/:driver_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DriversDelete),
		trackingController.DeleteDriver)
}

// registerRouteRoutes covers the board itself — the routes, their stops, and the alerts reported
// against them. Boarding and arrival are trip task transitions now (TRACK-008 D3).
func registerRouteRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Real-time operations
	r.POST("/alerts",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AlertsWrite),
		trackingController.CreateRouteAlert)

	// Routes
	r.GET("/routes", trackingController.ListRoutes)
	r.GET("/routes/:route_id", trackingController.GetRoute)
	r.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
	r.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
	r.POST("/routes",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateRoute)
	r.PATCH("/routes/:route_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.UpdateRoute)
	r.DELETE("/routes/:route_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
		trackingController.DeleteRoute)

	// Route versions — the history of what a route's stop list was, and the editor that changes it.
	r.GET("/routes/:route_id/versions",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListRouteVersions)
	r.POST("/routes/:route_id/versions",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateRouteVersion)
	r.PUT("/routes/:route_id/versions/:version_id/stops",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.ReplaceRouteVersionStops)
}

// registerTripRoutes covers the day's trips (TRACK-020): the operations board reads them like the
// route status, moving one is recording what happened (events:write), and overriding its vehicle,
// driver or start is editing the plan (routes:write). All of it is the operator's.
func registerTripRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	r.GET("/trips", denyOrganization, trackingController.ListTrips)
	r.GET("/trips/:trip_id", denyOrganization, trackingController.GetTrip)
	r.GET("/trips/:trip_id/status", denyOrganization, trackingController.GetTripStatus)
	r.PATCH("/trips/:trip_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.UpdateTrip)
	// :action is start|complete|cancel, arrive|skip and done|no-show; the handler answers 404 to any other.
	r.POST("/trips/:trip_id/:action",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.EventsWrite),
		trackingController.TransitionTrip)
	r.POST("/trip-stops/:stop_id/:action",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.EventsWrite),
		trackingController.TransitionTripStop)
	r.POST("/trip-stop-tasks/:task_id/:action",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.EventsWrite),
		trackingController.TransitionTripTask)
}

// registerStopPlaceRoutes covers the places routes call at (TRACK-007). A stop place is part of the
// board the operator plans, so it rides on the routes permissions rather than a family of its own.
func registerStopPlaceRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	r.GET("/stop-places",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListStopPlaces)
	r.GET("/stop-places/:stop_place_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.GetStopPlace)
	r.POST("/stop-places",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateStopPlace)
	r.PATCH("/stop-places/:stop_place_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.UpdateStopPlace)
	r.DELETE("/stop-places/:stop_place_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
		trackingController.DeleteStopPlace)
}

// registerScheduleRoutes covers when a route runs and what happens differently on one day
// (TRACK-018 D2). Planning the board is the operator's job, so these ride on the routes permissions
// rather than a family of their own.
func registerScheduleRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	r.GET("/routes/:route_id/schedules",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListRouteSchedules)
	r.POST("/routes/:route_id/schedules",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateRouteSchedule)
	r.PATCH("/routes/:route_id/schedules/:schedule_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.UpdateRouteSchedule)
	r.DELETE("/routes/:route_id/schedules/:schedule_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.DeleteRouteSchedule)
	// The split is a write of two schedules, not a delete of one: it keeps the routes-write
	// permission rather than routes-delete.
	r.POST("/routes/:route_id/schedules/:schedule_id/split",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.SplitRouteSchedule)
}

// registerCalendarRoutes covers the holidays and shutdowns a schedule reads (TRACK-018).
func registerCalendarRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	r.GET("/calendars",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListCalendars)
	r.POST("/calendars",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateCalendar)
	r.PATCH("/calendars/:calendar_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.UpdateCalendar)
	r.DELETE("/calendars/:calendar_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
		trackingController.DeleteCalendar)
	r.GET("/calendars/:calendar_id/dates",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListCalendarDates)
	r.PUT("/calendars/:calendar_id/dates",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.ReplaceCalendarDates)
}

// registerExceptionRoutes covers the days that do not follow the recurrence, and the read-only
// preview that puts schedules, calendar, version and exceptions together (TRACK-018).
func registerExceptionRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	r.GET("/routes/:route_id/exceptions",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.ListRouteExceptions)
	r.POST("/routes/:route_id/exceptions",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.CreateRouteException)
	r.DELETE("/routes/:route_id/exceptions/:exception_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
		trackingController.DeleteRouteException)
	r.GET("/routes/:route_id/preview",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.RoutesRead),
		trackingController.PreviewRoute)
}

// registerOrganizationRoutes covers the schools and employers, and who belongs to them.
func registerOrganizationRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Organizations
	r.GET("/organizations",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsRead),
		trackingController.ListOrganizations)
	r.GET("/organizations/:organization_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsRead),
		trackingController.GetOrganization)
	r.POST("/organizations",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite),
		trackingController.CreateOrganization)
	r.PATCH("/organizations/:organization_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite),
		trackingController.UpdateOrganization)
	r.DELETE("/organizations/:organization_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsDelete),
		trackingController.DeleteOrganization)

	// Organization members
	r.GET("/organizations/:organization_id/members",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsRead),
		trackingController.ListOrganizationMembers)
	r.PUT("/organizations/:organization_id/members/:user_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite),
		trackingController.UpsertOrganizationMember)
	r.DELETE("/organizations/:organization_id/members/:user_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite),
		trackingController.DeleteOrganizationMember)
}

// registerDocumentRoutes covers compliance documents and the policy behind them.
func registerDocumentRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Compliance documents and the policy behind them (TRACK-016). Reading them needs no
	// permission of its own — anyone who may see the fleet may see what it must carry; writing
	// and withdrawing share one permission.
	r.GET("/document-types", denyOrganization, trackingController.ListDocumentTypes)
	r.POST("/document-types",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.CreateDocumentType)
	r.PATCH("/document-types/:document_type_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.UpdateDocumentType)

	r.GET("/documents", denyOrganization, trackingController.ListDocuments)
	r.GET("/documents/:document_id", denyOrganization, trackingController.GetDocument)
	r.POST("/documents",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.CreateDocument)
	r.PATCH("/documents/:document_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.UpdateDocument)
	r.DELETE("/documents/:document_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.DeleteDocument)
	// The scan itself. Reading it is a read of the document; replacing it is a write of one.
	r.GET("/documents/:document_id/file", denyOrganization, trackingController.DownloadDocumentFile)
	r.PUT("/documents/:document_id/file",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.DocumentsWrite),
		trackingController.UploadDocumentFile)
}

// registerRiderRoutes is the only area a guardian reaches, and only through p.
func registerRiderRoutes(
	r *gin.RouterGroup,
	p *gin.RouterGroup,
	denyOrganization gin.HandlerFunc,
	trackingController *controllers.TrackingController,
) {
	// Riders. The list and a rider's status are the guardian's two reads (TRACK-017 D3), so they
	// hang off the portal-permissive chain; the scope a guardian sees is resolved in the SP, not
	// here. Everything else about a rider stays operator- and organization-only.
	p.GET("/riders", trackingController.ListRiders)
	p.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
	// Absences (TRACK-022) are a guardian's too: the SPs resolve the scope and mark a guardian's write
	// `portal`. A staff caller still needs riders:write to change them.
	p.GET("/riders/:rider_id/absences", trackingController.ListRiderAbsences)
	p.POST("/riders/:rider_id/absences",
		guardianOrPermission(trackingPerms.RidersWrite),
		trackingController.CreateRiderAbsence)
	p.DELETE("/riders/:rider_id/absences/:absence_id",
		guardianOrPermission(trackingPerms.RidersWrite),
		trackingController.DeleteRiderAbsence)
	r.GET("/riders/:rider_id/suggestions",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsRead),
		trackingController.SuggestRiderStops)
	r.GET("/riders/:rider_id", trackingController.GetRider)
	r.POST("/riders",
		tenancyMW.RequirePermission(trackingPerms.RidersWrite),
		trackingController.CreateRider)
	r.PATCH("/riders/:rider_id",
		tenancyMW.RequirePermission(trackingPerms.RidersWrite),
		trackingController.UpdateRider)
	r.DELETE("/riders/:rider_id",
		tenancyMW.RequirePermission(trackingPerms.RidersDelete),
		trackingController.DeleteRider)

	// Assignments (TRACK-009)
	r.GET("/assignments",
		tenancyMW.RequirePermission(trackingPerms.AssignmentsRead),
		trackingController.ListAssignments)
	r.POST("/assignments",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.CreateAssignment)
	r.POST("/assignments/bulk",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.CreateAssignmentsBulk)
	r.PATCH("/assignments/:assignment_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.UpdateAssignment)
	r.DELETE("/assignments/:assignment_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.DeleteAssignment)
}

// guardianOrPermission lets a portal account through — its scope is resolved in the SP — and holds
// every other level to permission.
func guardianOrPermission(permission string) gin.HandlerFunc {
	require := tenancyMW.RequirePermission(permission)
	return func(c *gin.Context) {
		if role, _ := c.Get("tenant_user_role"); role == tenancyModels.RolePortal {
			c.Next()
			return
		}
		require(c)
	}
}

// registerOpenRoutes is the single-database shape: no tenant, no permissions, every handler bare.
func registerOpenRoutes(trackingGroup *gin.RouterGroup, trackingController *controllers.TrackingController) {
	registerOpenFleetRoutes(trackingGroup, trackingController)
	registerOpenRouteRoutes(trackingGroup, trackingController)
	registerOpenStopPlaceRoutes(trackingGroup, trackingController)
	registerOpenScheduleRoutes(trackingGroup, trackingController)
	registerOpenCalendarRoutes(trackingGroup, trackingController)
	registerOpenExceptionRoutes(trackingGroup, trackingController)
	registerOpenOrganizationRoutes(trackingGroup, trackingController)
	registerOpenDocumentRoutes(trackingGroup, trackingController)
	registerOpenRiderRoutes(trackingGroup, trackingController)
	registerOpenTripRoutes(trackingGroup, trackingController)
}

// registerOpenFleetRoutes mirrors registerFleetRoutes without the guards.
func registerOpenFleetRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.POST("/companies", trackingController.CreateCompany)
	g.PATCH("/companies/:company_id", trackingController.UpdateCompany)
	g.GET("/companies", trackingController.ListCompanies)
	g.GET("/companies/:company_id", trackingController.GetCompany)
	g.DELETE("/companies/:company_id", trackingController.DeleteCompany)
	g.POST("/vehicles", trackingController.CreateVehicle)
	g.GET("/vehicles", trackingController.ListVehicles)
	g.GET("/vehicles/:vehicle_id", trackingController.GetVehicle)
	g.GET("/vehicles/:vehicle_id/location", trackingController.GetVehicleCurrentLocation)
	g.PATCH("/vehicles/:vehicle_id", trackingController.UpdateVehicle)
	g.DELETE("/vehicles/:vehicle_id", trackingController.DeleteVehicle)
	g.POST("/drivers", trackingController.CreateDriver)
	g.GET("/drivers", trackingController.ListDrivers)
	g.GET("/drivers/:driver_id", trackingController.GetDriver)
	g.PATCH("/drivers/:driver_id", trackingController.UpdateDriver)
	g.DELETE("/drivers/:driver_id", trackingController.DeleteDriver)
}

// registerOpenRouteRoutes mirrors registerRouteRoutes without the guards.
func registerOpenRouteRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.POST("/alerts", trackingController.CreateRouteAlert)
	g.POST("/routes", trackingController.CreateRoute)
	g.GET("/routes", trackingController.ListRoutes)
	g.GET("/routes/:route_id", trackingController.GetRoute)
	g.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
	g.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
	g.PATCH("/routes/:route_id", trackingController.UpdateRoute)
	g.DELETE("/routes/:route_id", trackingController.DeleteRoute)
	g.GET("/routes/:route_id/versions", trackingController.ListRouteVersions)
	g.POST("/routes/:route_id/versions", trackingController.CreateRouteVersion)
	g.PUT("/routes/:route_id/versions/:version_id/stops", trackingController.ReplaceRouteVersionStops)
}

// registerOpenStopPlaceRoutes mirrors registerStopPlaceRoutes without the guards.
func registerOpenStopPlaceRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/stop-places", trackingController.ListStopPlaces)
	g.GET("/stop-places/:stop_place_id", trackingController.GetStopPlace)
	g.POST("/stop-places", trackingController.CreateStopPlace)
	g.PATCH("/stop-places/:stop_place_id", trackingController.UpdateStopPlace)
	g.DELETE("/stop-places/:stop_place_id", trackingController.DeleteStopPlace)
}

// registerOpenScheduleRoutes mirrors registerScheduleRoutes without the guards.
func registerOpenScheduleRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/routes/:route_id/schedules", trackingController.ListRouteSchedules)
	g.POST("/routes/:route_id/schedules", trackingController.CreateRouteSchedule)
	g.PATCH("/routes/:route_id/schedules/:schedule_id", trackingController.UpdateRouteSchedule)
	g.DELETE("/routes/:route_id/schedules/:schedule_id", trackingController.DeleteRouteSchedule)
	g.POST("/routes/:route_id/schedules/:schedule_id/split", trackingController.SplitRouteSchedule)
}

// registerOpenCalendarRoutes mirrors registerCalendarRoutes without the guards.
func registerOpenCalendarRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/calendars", trackingController.ListCalendars)
	g.POST("/calendars", trackingController.CreateCalendar)
	g.PATCH("/calendars/:calendar_id", trackingController.UpdateCalendar)
	g.DELETE("/calendars/:calendar_id", trackingController.DeleteCalendar)
	g.GET("/calendars/:calendar_id/dates", trackingController.ListCalendarDates)
	g.PUT("/calendars/:calendar_id/dates", trackingController.ReplaceCalendarDates)
}

// registerOpenExceptionRoutes mirrors registerExceptionRoutes without the guards.
func registerOpenExceptionRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/routes/:route_id/exceptions", trackingController.ListRouteExceptions)
	g.POST("/routes/:route_id/exceptions", trackingController.CreateRouteException)
	g.DELETE("/routes/:route_id/exceptions/:exception_id", trackingController.DeleteRouteException)
	g.GET("/routes/:route_id/preview", trackingController.PreviewRoute)
}

// registerOpenOrganizationRoutes mirrors registerOrganizationRoutes without the guards.
func registerOpenOrganizationRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.POST("/organizations", trackingController.CreateOrganization)
	g.GET("/organizations", trackingController.ListOrganizations)
	g.GET("/organizations/:organization_id", trackingController.GetOrganization)
	g.PATCH("/organizations/:organization_id", trackingController.UpdateOrganization)
	g.DELETE("/organizations/:organization_id", trackingController.DeleteOrganization)
	g.GET("/organizations/:organization_id/members", trackingController.ListOrganizationMembers)
	g.PUT("/organizations/:organization_id/members/:user_id", trackingController.UpsertOrganizationMember)
	g.DELETE("/organizations/:organization_id/members/:user_id", trackingController.DeleteOrganizationMember)
}

// registerOpenDocumentRoutes mirrors registerDocumentRoutes without the guards.
func registerOpenDocumentRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/document-types", trackingController.ListDocumentTypes)
	g.POST("/document-types", trackingController.CreateDocumentType)
	g.PATCH("/document-types/:document_type_id", trackingController.UpdateDocumentType)
	g.GET("/documents", trackingController.ListDocuments)
	g.GET("/documents/:document_id", trackingController.GetDocument)
	g.POST("/documents", trackingController.CreateDocument)
	g.PATCH("/documents/:document_id", trackingController.UpdateDocument)
	g.DELETE("/documents/:document_id", trackingController.DeleteDocument)
	g.GET("/documents/:document_id/file", trackingController.DownloadDocumentFile)
	g.PUT("/documents/:document_id/file", trackingController.UploadDocumentFile)
}

// registerOpenRiderRoutes mirrors registerRiderRoutes without the guards.
func registerOpenRiderRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.POST("/riders", trackingController.CreateRider)
	g.GET("/riders", trackingController.ListRiders)
	g.GET("/riders/:rider_id", trackingController.GetRider)
	g.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
	g.PATCH("/riders/:rider_id", trackingController.UpdateRider)
	g.DELETE("/riders/:rider_id", trackingController.DeleteRider)
	g.GET("/riders/:rider_id/absences", trackingController.ListRiderAbsences)
	g.POST("/riders/:rider_id/absences", trackingController.CreateRiderAbsence)
	g.DELETE("/riders/:rider_id/absences/:absence_id", trackingController.DeleteRiderAbsence)
	g.GET("/riders/:rider_id/suggestions", trackingController.SuggestRiderStops)
	g.GET("/assignments", trackingController.ListAssignments)
	g.POST("/assignments", trackingController.CreateAssignment)
	g.POST("/assignments/bulk", trackingController.CreateAssignmentsBulk)
	g.PATCH("/assignments/:assignment_id", trackingController.UpdateAssignment)
	g.DELETE("/assignments/:assignment_id", trackingController.DeleteAssignment)
}

// registerOpenTripRoutes mirrors registerTripRoutes without the guards.
func registerOpenTripRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	g.GET("/trips", trackingController.ListTrips)
	g.GET("/trips/:trip_id", trackingController.GetTrip)
	g.GET("/trips/:trip_id/status", trackingController.GetTripStatus)
	g.PATCH("/trips/:trip_id", trackingController.UpdateTrip)
	g.POST("/trips/:trip_id/:action", trackingController.TransitionTrip)
	g.POST("/trip-stops/:stop_id/:action", trackingController.TransitionTripStop)
	g.POST("/trip-stop-tasks/:task_id/:action", trackingController.TransitionTripTask)
}
