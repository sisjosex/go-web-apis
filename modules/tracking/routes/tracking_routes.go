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
	registerOrganizationRoutes(r, denyOrganization, trackingController)
	registerDocumentRoutes(r, denyOrganization, trackingController)
	registerRiderRoutes(r, p, denyOrganization, trackingController)
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

// registerRouteRoutes covers the board itself — the routes, their stops, and the events and alerts
// reported against them.
func registerRouteRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Real-time operations
	r.POST("/events",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.EventsWrite),
		trackingController.RecordRideEvent)
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

	// Assignments
	r.GET("/assignments",
		tenancyMW.RequirePermission(trackingPerms.AssignmentsRead),
		trackingController.ListRiderAssignments)
	r.POST("/assignments",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.AssignRider)
	r.DELETE("/assignments/:assignment_id",
		denyOrganization,
		tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
		trackingController.UnassignRider)
}

// registerOpenRoutes is the single-database shape: no tenant, no permissions, every handler bare.
func registerOpenRoutes(trackingGroup *gin.RouterGroup, trackingController *controllers.TrackingController) {
	registerOpenFleetRoutes(trackingGroup, trackingController)
	registerOpenRouteRoutes(trackingGroup, trackingController)
	registerOpenStopPlaceRoutes(trackingGroup, trackingController)
	registerOpenOrganizationRoutes(trackingGroup, trackingController)
	registerOpenDocumentRoutes(trackingGroup, trackingController)
	registerOpenRiderRoutes(trackingGroup, trackingController)
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
	g.POST("/events", trackingController.RecordRideEvent)
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
	g.POST("/assignments", trackingController.AssignRider)
	g.GET("/assignments", trackingController.ListRiderAssignments)
	g.DELETE("/assignments/:assignment_id", trackingController.UnassignRider)
}
