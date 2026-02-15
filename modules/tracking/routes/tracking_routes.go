package routes

import (
	"josex/web/config"
	"josex/web/modules/auth/middleware"
	authServices "josex/web/modules/auth/services"
	"josex/web/modules/tracking/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterTrackingRoutes registers all tracking module routes
// Routes work in both single-database and multi-tenant modes
// Controllers handle tenant validation based on config
// jwtService is required for authentication middleware on protected routes
func RegisterTrackingRoutes(router *gin.Engine, trackingController *controllers.TrackingController, tenantMiddleware gin.HandlerFunc, jwtService authServices.JWTService) {
	trackingGroup := router.Group("/api/v1/tracking")

	// Public routes (GPS device updates - no auth required)
	trackingGroup.POST("/locations", trackingController.UpdateVehicleLocation)

	// Check if multi-tenant mode is enabled
	coreConf := config.ModularAppConfig.Core
	tenancyConf := config.ModularAppConfig.Tenancy

	if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
		// Multi-tenant mode: create separate group with auth + tenant middleware
		protectedRoutes := trackingGroup.Group("")
		// Apply auth middleware first (sets system_role in context)
		protectedRoutes.Use(middleware.AuthMiddleware(jwtService))
		// Then apply tenant middleware (uses system_role from context)
		protectedRoutes.Use(tenantMiddleware)

		// Real-time operations
		protectedRoutes.POST("/events", trackingController.RecordRideEvent)
		protectedRoutes.POST("/alerts", trackingController.CreateRouteAlert)

		// Companies CRUD
		protectedRoutes.POST("/companies", trackingController.CreateCompany)
		protectedRoutes.PATCH("/companies/:company_id", trackingController.UpdateCompany)
		protectedRoutes.GET("/companies", trackingController.ListCompanies)
		protectedRoutes.GET("/companies/:company_id", trackingController.GetCompany)
		protectedRoutes.DELETE("/companies/:company_id", trackingController.DeleteCompany)

		// Vehicles CRUD
		protectedRoutes.POST("/vehicles", trackingController.CreateVehicle)
		protectedRoutes.GET("/vehicles", trackingController.ListVehicles)
		protectedRoutes.GET("/vehicles/:vehicle_id", trackingController.GetVehicle)
		protectedRoutes.GET("/vehicles/:vehicle_id/location", trackingController.GetVehicleCurrentLocation)
		protectedRoutes.PATCH("/vehicles/:vehicle_id", trackingController.UpdateVehicle)
		protectedRoutes.DELETE("/vehicles/:vehicle_id", trackingController.DeleteVehicle)

		// Routes CRUD
		protectedRoutes.POST("/routes", trackingController.CreateRoute)
		protectedRoutes.GET("/routes", trackingController.ListRoutes)
		protectedRoutes.GET("/routes/:route_id", trackingController.GetRoute)
		protectedRoutes.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
		protectedRoutes.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
		protectedRoutes.PATCH("/routes/:route_id", trackingController.UpdateRoute)
		protectedRoutes.DELETE("/routes/:route_id", trackingController.DeleteRoute)

		// Route Stops CRUD
		protectedRoutes.POST("/route-stops", trackingController.CreateRouteStop)
		protectedRoutes.DELETE("/route-stops/:stop_id", trackingController.DeleteRouteStop)

		// Riders CRUD
		protectedRoutes.POST("/riders", trackingController.CreateRider)
		protectedRoutes.GET("/companies/:company_id/riders", trackingController.ListRiders)
		protectedRoutes.GET("/riders/:rider_id", trackingController.GetRider)
		protectedRoutes.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
		protectedRoutes.PATCH("/riders/:rider_id", trackingController.UpdateRider)
		protectedRoutes.DELETE("/riders/:rider_id", trackingController.DeleteRider)

		// Rider Assignments
		protectedRoutes.POST("/assignments", trackingController.AssignRider)
		protectedRoutes.GET("/assignments", trackingController.ListRiderAssignments)
		protectedRoutes.DELETE("/assignments/:assignment_id", trackingController.UnassignRider)

		// Client Access Management
		protectedRoutes.POST("/companies/:company_id/clients", trackingController.GrantClientAccess)
		protectedRoutes.GET("/companies/:company_id/clients", trackingController.ListCompanyClients)
		protectedRoutes.DELETE("/companies/:company_id/clients/:client_tenant_id", trackingController.RevokeClientAccess)
	} else {
		// Single-database mode: no tenant middleware, all routes use trackingGroup
		// Real-time operations
		trackingGroup.POST("/events", trackingController.RecordRideEvent)
		trackingGroup.POST("/alerts", trackingController.CreateRouteAlert)

		// Companies CRUD
		trackingGroup.POST("/companies", trackingController.CreateCompany)
		trackingGroup.PATCH("/companies/:company_id", trackingController.UpdateCompany)
		trackingGroup.GET("/companies", trackingController.ListCompanies)
		trackingGroup.GET("/companies/:company_id", trackingController.GetCompany)
		trackingGroup.DELETE("/companies/:company_id", trackingController.DeleteCompany)

		// Vehicles CRUD
		trackingGroup.POST("/vehicles", trackingController.CreateVehicle)
		trackingGroup.GET("/vehicles", trackingController.ListVehicles)
		trackingGroup.GET("/vehicles/:vehicle_id", trackingController.GetVehicle)
		trackingGroup.GET("/vehicles/:vehicle_id/location", trackingController.GetVehicleCurrentLocation)
		trackingGroup.PATCH("/vehicles/:vehicle_id", trackingController.UpdateVehicle)
		trackingGroup.DELETE("/vehicles/:vehicle_id", trackingController.DeleteVehicle)

		// Routes CRUD
		trackingGroup.POST("/routes", trackingController.CreateRoute)
		trackingGroup.GET("/routes", trackingController.ListRoutes)
		trackingGroup.GET("/routes/:route_id", trackingController.GetRoute)
		trackingGroup.GET("/routes/:route_id/status", trackingController.GetRouteRealtimeStatus)
		trackingGroup.GET("/routes/:route_id/stops", trackingController.ListRouteStops)
		trackingGroup.PATCH("/routes/:route_id", trackingController.UpdateRoute)
		trackingGroup.DELETE("/routes/:route_id", trackingController.DeleteRoute)

		// Route Stops CRUD
		trackingGroup.POST("/route-stops", trackingController.CreateRouteStop)
		trackingGroup.DELETE("/route-stops/:stop_id", trackingController.DeleteRouteStop)

		// Riders CRUD
		trackingGroup.POST("/riders", trackingController.CreateRider)
		trackingGroup.GET("/companies/:company_id/riders", trackingController.ListRiders)
		trackingGroup.GET("/riders/:rider_id", trackingController.GetRider)
		trackingGroup.GET("/riders/:rider_id/status", trackingController.GetRiderStatus)
		trackingGroup.PATCH("/riders/:rider_id", trackingController.UpdateRider)
		trackingGroup.DELETE("/riders/:rider_id", trackingController.DeleteRider)

		// Rider Assignments
		trackingGroup.POST("/assignments", trackingController.AssignRider)
		trackingGroup.GET("/assignments", trackingController.ListRiderAssignments)
		trackingGroup.DELETE("/assignments/:assignment_id", trackingController.UnassignRider)

		// Client Access Management
		trackingGroup.POST("/companies/:company_id/clients", trackingController.GrantClientAccess)
		trackingGroup.GET("/companies/:company_id/clients", trackingController.ListCompanyClients)
		trackingGroup.DELETE("/companies/:company_id/clients/:client_tenant_id", trackingController.RevokeClientAccess)
	}
}
