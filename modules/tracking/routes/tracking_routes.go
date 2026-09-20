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
func RegisterTrackingRoutes(router *gin.Engine, trackingController *controllers.TrackingController, tenantMiddleware gin.HandlerFunc, jwtService authServices.JWTService) {
	trackingGroup := router.Group("/api/v1/tracking")

	// Check if multi-tenant mode is enabled
	coreConf := config.ModularAppConfig.Core
	tenancyConf := config.ModularAppConfig.Tenancy

	if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
		// Multi-tenant mode: auth + tenant middleware + per-route permission checks
		r := trackingGroup.Group("")
		r.Use(middleware.AuthMiddleware(jwtService))
		r.Use(tenantMiddleware)

		// An organization user sees its own people and the routes they ride, and edits those
		// riders — nothing else (TRACK-015 D1). The fleet, the organizations themselves and every
		// write outside riders are the operator's, so those routes deny the level outright rather
		// than relying on a scope the SP behind them does not take.
		denyOrganization := tenancyMW.DenyTenantRole(tenancyModels.RoleOrganization)

		// Real-time operations
		r.POST("/events",
			denyOrganization,
			tenancyMW.RequirePermission(trackingPerms.EventsWrite),
			trackingController.RecordRideEvent)
		r.POST("/alerts",
			denyOrganization,
			tenancyMW.RequirePermission(trackingPerms.AlertsWrite),
			trackingController.CreateRouteAlert)

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

		// Route Stops
		r.POST("/route-stops",
			denyOrganization,
			tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
			trackingController.CreateRouteStop)
		r.DELETE("/route-stops/:stop_id",
			denyOrganization,
			tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
			trackingController.DeleteRouteStop)

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

		// Riders
		r.GET("/riders", trackingController.ListRiders)
		r.GET("/riders/:rider_id", trackingController.GetRider)
		r.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
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
	} else {
		// Single-database mode: no tenant/permission middleware
		trackingGroup.POST("/events", trackingController.RecordRideEvent)
		trackingGroup.POST("/alerts", trackingController.CreateRouteAlert)

		trackingGroup.POST("/companies", trackingController.CreateCompany)
		trackingGroup.PATCH("/companies/:company_id", trackingController.UpdateCompany)
		trackingGroup.GET("/companies", trackingController.ListCompanies)
		trackingGroup.GET("/companies/:company_id", trackingController.GetCompany)
		trackingGroup.DELETE("/companies/:company_id", trackingController.DeleteCompany)

		trackingGroup.POST("/vehicles", trackingController.CreateVehicle)
		trackingGroup.GET("/vehicles", trackingController.ListVehicles)
		trackingGroup.GET("/vehicles/:vehicle_id", trackingController.GetVehicle)
		trackingGroup.GET("/vehicles/:vehicle_id/location", trackingController.GetVehicleCurrentLocation)
		trackingGroup.PATCH("/vehicles/:vehicle_id", trackingController.UpdateVehicle)
		trackingGroup.DELETE("/vehicles/:vehicle_id", trackingController.DeleteVehicle)

		trackingGroup.POST("/routes", trackingController.CreateRoute)
		trackingGroup.GET("/routes", trackingController.ListRoutes)
		trackingGroup.GET("/routes/:route_id", trackingController.GetRoute)
		trackingGroup.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
		trackingGroup.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
		trackingGroup.PATCH("/routes/:route_id", trackingController.UpdateRoute)
		trackingGroup.DELETE("/routes/:route_id", trackingController.DeleteRoute)

		trackingGroup.POST("/route-stops", trackingController.CreateRouteStop)
		trackingGroup.DELETE("/route-stops/:stop_id", trackingController.DeleteRouteStop)

		trackingGroup.POST("/organizations", trackingController.CreateOrganization)
		trackingGroup.GET("/organizations", trackingController.ListOrganizations)
		trackingGroup.GET("/organizations/:organization_id", trackingController.GetOrganization)
		trackingGroup.PATCH("/organizations/:organization_id", trackingController.UpdateOrganization)
		trackingGroup.DELETE("/organizations/:organization_id", trackingController.DeleteOrganization)

		trackingGroup.GET("/organizations/:organization_id/members", trackingController.ListOrganizationMembers)
		trackingGroup.PUT("/organizations/:organization_id/members/:user_id", trackingController.UpsertOrganizationMember)
		trackingGroup.DELETE("/organizations/:organization_id/members/:user_id", trackingController.DeleteOrganizationMember)

		trackingGroup.POST("/drivers", trackingController.CreateDriver)
		trackingGroup.GET("/drivers", trackingController.ListDrivers)
		trackingGroup.GET("/drivers/:driver_id", trackingController.GetDriver)
		trackingGroup.PATCH("/drivers/:driver_id", trackingController.UpdateDriver)
		trackingGroup.DELETE("/drivers/:driver_id", trackingController.DeleteDriver)

		trackingGroup.GET("/document-types", trackingController.ListDocumentTypes)
		trackingGroup.POST("/document-types", trackingController.CreateDocumentType)
		trackingGroup.PATCH("/document-types/:document_type_id", trackingController.UpdateDocumentType)

		trackingGroup.GET("/documents", trackingController.ListDocuments)
		trackingGroup.GET("/documents/:document_id", trackingController.GetDocument)
		trackingGroup.POST("/documents", trackingController.CreateDocument)
		trackingGroup.PATCH("/documents/:document_id", trackingController.UpdateDocument)
		trackingGroup.DELETE("/documents/:document_id", trackingController.DeleteDocument)
		trackingGroup.GET("/documents/:document_id/file", trackingController.DownloadDocumentFile)
		trackingGroup.PUT("/documents/:document_id/file", trackingController.UploadDocumentFile)

		trackingGroup.POST("/riders", trackingController.CreateRider)
		trackingGroup.GET("/riders", trackingController.ListRiders)
		trackingGroup.GET("/riders/:rider_id", trackingController.GetRider)
		trackingGroup.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
		trackingGroup.PATCH("/riders/:rider_id", trackingController.UpdateRider)
		trackingGroup.DELETE("/riders/:rider_id", trackingController.DeleteRider)

		trackingGroup.POST("/assignments", trackingController.AssignRider)
		trackingGroup.GET("/assignments", trackingController.ListRiderAssignments)
		trackingGroup.DELETE("/assignments/:assignment_id", trackingController.UnassignRider)
	}
}
