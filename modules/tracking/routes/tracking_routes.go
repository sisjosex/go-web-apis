package routes

import (
	"josex/web/config"
	"josex/web/modules/auth/middleware"
	authServices "josex/web/modules/auth/services"
	"josex/web/modules/tracking/controllers"
	trackingPerms "josex/web/modules/tracking/permissions"
	tenancyMW "josex/web/modules/tenancy/middleware"

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

		// Real-time operations
		r.POST("/events",
			tenancyMW.RequirePermission(trackingPerms.EventsWrite),
			trackingController.RecordRideEvent)
		r.POST("/alerts",
			tenancyMW.RequirePermission(trackingPerms.AlertsWrite),
			trackingController.CreateRouteAlert)

		// Companies
		r.GET("/companies", trackingController.ListCompanies)
		r.GET("/companies/:company_id", trackingController.GetCompany)
		r.POST("/companies",
			tenancyMW.RequirePermission(trackingPerms.CompaniesWrite),
			trackingController.CreateCompany)
		r.PATCH("/companies/:company_id",
			tenancyMW.RequirePermission(trackingPerms.CompaniesWrite),
			trackingController.UpdateCompany)
		r.DELETE("/companies/:company_id",
			tenancyMW.RequirePermission(trackingPerms.CompaniesDelete),
			trackingController.DeleteCompany)

		// Vehicles
		r.GET("/vehicles", trackingController.ListVehicles)
		r.GET("/vehicles/:vehicle_id", trackingController.GetVehicle)
		r.GET("/vehicles/:vehicle_id/location", trackingController.GetVehicleCurrentLocation)
		r.POST("/vehicles",
			tenancyMW.RequirePermission(trackingPerms.VehiclesWrite),
			trackingController.CreateVehicle)
		r.PATCH("/vehicles/:vehicle_id",
			tenancyMW.RequirePermission(trackingPerms.VehiclesWrite),
			trackingController.UpdateVehicle)
		r.DELETE("/vehicles/:vehicle_id",
			tenancyMW.RequirePermission(trackingPerms.VehiclesDelete),
			trackingController.DeleteVehicle)

		// Routes
		r.GET("/routes", trackingController.ListRoutes)
		r.GET("/routes/:route_id", trackingController.GetRoute)
		r.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
		r.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
		r.POST("/routes",
			tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
			trackingController.CreateRoute)
		r.PATCH("/routes/:route_id",
			tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
			trackingController.UpdateRoute)
		r.DELETE("/routes/:route_id",
			tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
			trackingController.DeleteRoute)

		// Route Stops
		r.POST("/route-stops",
			tenancyMW.RequirePermission(trackingPerms.RoutesWrite),
			trackingController.CreateRouteStop)
		r.DELETE("/route-stops/:stop_id",
			tenancyMW.RequirePermission(trackingPerms.RoutesDelete),
			trackingController.DeleteRouteStop)

		// Riders
		r.GET("/companies/:company_id/riders", trackingController.ListRiders)
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
			tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
			trackingController.AssignRider)
		r.DELETE("/assignments/:assignment_id",
			tenancyMW.RequirePermission(trackingPerms.AssignmentsManage),
			trackingController.UnassignRider)

		// Client Access Management
		r.GET("/companies/:company_id/clients",
			tenancyMW.RequirePermission(trackingPerms.ClientsManage),
			trackingController.ListCompanyClients)
		r.POST("/companies/:company_id/clients",
			tenancyMW.RequirePermission(trackingPerms.ClientsManage),
			trackingController.GrantClientAccess)
		r.DELETE("/companies/:company_id/clients/:client_tenant_id",
			tenancyMW.RequirePermission(trackingPerms.ClientsManage),
			trackingController.RevokeClientAccess)
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

		trackingGroup.POST("/riders", trackingController.CreateRider)
		trackingGroup.GET("/companies/:company_id/riders", trackingController.ListRiders)
		trackingGroup.GET("/riders/:rider_id", trackingController.GetRider)
		trackingGroup.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
		trackingGroup.PATCH("/riders/:rider_id", trackingController.UpdateRider)
		trackingGroup.DELETE("/riders/:rider_id", trackingController.DeleteRider)

		trackingGroup.POST("/assignments", trackingController.AssignRider)
		trackingGroup.GET("/assignments", trackingController.ListRiderAssignments)
		trackingGroup.DELETE("/assignments/:assignment_id", trackingController.UnassignRider)

		trackingGroup.POST("/companies/:company_id/clients", trackingController.GrantClientAccess)
		trackingGroup.GET("/companies/:company_id/clients", trackingController.ListCompanyClients)
		trackingGroup.DELETE("/companies/:company_id/clients/:client_tenant_id", trackingController.RevokeClientAccess)
	}
}
