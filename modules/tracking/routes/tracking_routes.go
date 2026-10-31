package routes

import (
	"josex/web/config"
	"josex/web/modules/auth/middleware"
	authServices "josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	tenancyMW "josex/web/modules/tenancy/middleware"
	tenancyModels "josex/web/modules/tenancy/models"
	"josex/web/modules/tracking/controllers"
	trackingPerms "josex/web/modules/tracking/permissions"

	"github.com/gin-gonic/gin"
)

// Ingest is the GPS ingest route (TRACK-010): its controller and the chain that authenticates either a
// device token or a driver. Auth nil — no tenancy — leaves the route unregistered: a device token
// names its tenant, which needs the platform database.
// Device is the hardware tracker's chain alone, for the OsmAnd route (TRACK-068 D1).
type Ingest struct {
	Controller *controllers.IngestController
	Auth       []gin.HandlerFunc
	Device     []gin.HandlerFunc
	// Tokens issues a vehicle's device token from its page (TRACK-068); nil without tenancy.
	Tokens *controllers.GPSDeviceController
}

// Driver is the driver's phone (TRACK-011): its controller and the ingest's driver chain — auth, the
// tenant chain with the driver's mobile-only refusal lifted, the module check. Auth nil — no tenancy —
// leaves the group unregistered.
type Driver struct {
	Controller *controllers.DriverController
	Auth       []gin.HandlerFunc
}

// Portal is the guardian's phone (TRACK-012): its controller and the portal chain — auth, the tenant
// chain with the guardian's mobile-only refusal lifted, the module check. Auth nil — no tenancy —
// leaves the group unregistered.
type Portal struct {
	Controller *controllers.PortalController
	Auth       []gin.HandlerFunc
	// AuthOnly is /mobile/portal/me's chain (TRACK-061 D1): a session and no tenant, since the answer
	// spans every operator the account is a guardian in. Nil leaves the route unregistered.
	AuthOnly []gin.HandlerFunc
	// Reads are the rider's day, history and absence cut-offs, on the portal-permissive chain.
	Reads *controllers.RiderReadsController
}

// Live is the realtime surface (TRACK-025): the snapshot reads, and the WebSocket endpoint's whole
// handler list (browser auth, auth, tenant chain, module check, upgrade) — nil when this process
// runs no gateway.
type Live struct {
	Controller *controllers.LiveController
	Socket     []gin.HandlerFunc
	// Public is the share page's chain (TRACK-060): the token's slug names the tenant, no session.
	Public []gin.HandlerFunc
}

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
	ingest Ingest,
	live Live,
	driver Driver,
	portal Portal,
	access *controllers.AccessController,
) {
	trackingGroup := router.Group("/api/v1/tracking")

	// Check if multi-tenant mode is enabled
	coreConf := config.ModularAppConfig.Core
	tenancyConf := config.ModularAppConfig.Tenancy

	if coreConf.IsModuleEnabled("tenancy") && tenancyConf != nil && tenancyConf.Enabled {
		registerTenantRoutes(trackingGroup, trackingController, tenantMiddleware, portalTenantMiddleware, jwtService, live.Controller, portal.Reads, access, ingest.Tokens)
		// Outside the groups: each carries its own chain — a device token or a driver's session for the
		// ingest, a browser's subprotocol token and ?tenant_slug= for the socket.
		if ingest.Auth != nil {
			api.Post(trackingGroup, "/ingest/positions", ingest.Controller.IngestPositions, api.Doc{Summary: "Post a batch of GPS points", Description: "Up to 500 points, any order, late ones included. Authenticated either by X-Device-Token (a GPS device: its vehicle) or by a driver's Bearer + X-Tenant-Slug (the vehicle of their trip in progress). Replaying a point is a no-op. 202 path=queued (the worker stores it) or stored (Valkey absent: stored inline)", Status: 202, Errors: []int{400, 401, 409, 500}}, ingest.Auth...)
		}
		if ingest.Device != nil {
			osmand := api.Doc{Summary: "One point of a hardware tracker (OsmAnd protocol)", Description: "Traccar Client, OsmAnd and trackers that speak the OsmAnd query protocol: id is the device token issued from the vehicle page, lat, lon, timestamp (unix seconds or RFC3339, now when absent), speed in knots, bearing, accuracy; altitude and batt are ignored. During a trip the point yields to a phone that reported in the last GPS_PHONE_PRIORITY_S; off a trip it only moves the last position. 200 with an empty body", Status: 200, Errors: []int{400, 401, 429, 500}}
			api.Get(&router.RouterGroup, "/api/v1/gps/osmand", ingest.Controller.IngestOsmand, osmand, ingest.Device...)
			api.Post(&router.RouterGroup, "/api/v1/gps/osmand", ingest.Controller.IngestOsmand, osmand, ingest.Device...)
		}
		if live.Socket != nil {
			trackingGroup.GET("/ws", live.Socket...)
		}
		if live.Public != nil {
			api.Get(&router.RouterGroup, "/api/v1/public/share/{token}", live.Controller.GetSharedTrip, api.Doc{Summary: "A shared trip, for anyone holding the link", Description: "No session: the token's slug names the operator. Never a driver's or passenger's name; an unknown, expired or revoked link is one 404.", Errors: []int{404, 429}}, live.Public...)
		}
		if driver.Auth != nil {
			registerDriverRoutes(router, driver)
		}
		if portal.Auth != nil {
			registerPortalRoutes(router, portal)
		}
		if portal.AuthOnly != nil {
			api.Get(&router.RouterGroup, "/api/v1/mobile/portal/me", portal.Controller.Me, api.Doc{Summary: "The account's riders in every operator", Description: "The guardian's home across operators (TRACK-061 D1): the account, and per tenant where it holds the portal role (newest membership first) its riders as GET /tracking/riders/today answers them. One operator's read failing marks that entry error with riders null; the others still answer. Bearer only, no X-Tenant-Slug", Errors: []int{403, 500}}, portal.AuthOnly...)
		}
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
	liveController *controllers.LiveController,
	riderReads *controllers.RiderReadsController,
	access *controllers.AccessController,
	devices *controllers.GPSDeviceController,
) {
	// Multi-tenant mode: auth + tenant middleware + per-route permission checks
	r := trackingGroup.Group("")
	r.Use(middleware.AuthMiddleware(jwtService))
	r.Use(tenantMiddleware)

	// An organization user sees its own people and the routes they ride, and edits those
	// riders — nothing else (TRACK-015 D1) — plus its own organization, its members and its live
	// map (TRACK-067 D1). The rest of the fleet and every other write are the operator's, so those
	// routes deny the level outright rather than relying on a scope the SP behind them does not take.
	denyOrganization := tenancyMW.DenyTenantRole(tenancyModels.RoleOrganization)

	// The guardian's group: auth, then the chain that lets a portal account through.
	p := trackingGroup.Group("")
	p.Use(middleware.AuthMiddleware(jwtService))
	p.Use(portalTenantMiddleware)

	registerFleetRoutes(r, denyOrganization, trackingController)
	if devices != nil {
		registerGPSDeviceRoutes(r, denyOrganization, devices)
	}
	registerRouteRoutes(r, denyOrganization, trackingController)
	registerStopPlaceRoutes(r, denyOrganization, trackingController)
	registerScheduleRoutes(r, denyOrganization, trackingController)
	registerCalendarRoutes(r, denyOrganization, trackingController)
	registerExceptionRoutes(r, denyOrganization, trackingController)
	registerOrganizationRoutes(r, denyOrganization, trackingController, access)
	registerDocumentRoutes(r, denyOrganization, trackingController)
	registerRiderRoutes(r, p, denyOrganization, trackingController, riderReads)
	registerTripRoutes(r, denyOrganization, trackingController)
	registerLiveRoutes(r, p, liveController)
	registerAccessRoutes(r, denyOrganization, access)
	registerPlanningRoutes(r, denyOrganization, trackingController)
	registerReportRoutes(r, trackingController)
}

// registerGPSDeviceRoutes is a vehicle's tracker token, from its page (TRACK-068): read by whoever
// sees the fleet, issued, rotated and revoked with vehicles:write.
func registerGPSDeviceRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, devices *controllers.GPSDeviceController) {
	api.Get(r, "/vehicles/{vehicle_id}/gps-device", devices.GPSDeviceStatus, api.Doc{Summary: "The vehicle's GPS device", Description: "Whether a tracker is linked, when its token was issued and when it last posted (null before its first point).", Errors: []int{400, 404}}, denyOrganization)
	api.Post(r, "/vehicles/{vehicle_id}/gps-device", devices.IssueGPSDevice, api.Doc{Summary: "Issue or rotate the vehicle's GPS device token", Description: "A new token for the tracker (the OsmAnd id); the previous one answers 401 from now on. The token is answered this once — only its hash is stored.", Status: 201, Errors: []int{400, 404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
	api.Delete(r, "/vehicles/{vehicle_id}/gps-device", devices.RevokeGPSDevice, api.Doc{Summary: "Unlink the vehicle's GPS device", Description: "Its token answers 401 from now on; a vehicle without one is a no-op.", Status: 204, Errors: []int{400, 404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
}

// registerReportRoutes is the reports (TRACK-013): their own permission (D4), and an organization user
// reaches them too — each SP narrows the rows to its organizations.
func registerReportRoutes(r *gin.RouterGroup, trackingController *controllers.TrackingController) {
	reports := r.Group("/reports", tenancyMW.RequirePermission(trackingPerms.ReportsRead))
	api.Get(reports, "/attendance", trackingController.ReportAttendance, api.Doc{Summary: "Attendance report", Description: "Riders with their boarded, no-show, absent and cancelled rides between from and to (default the last 30 days, at most 366), least attended first. attendance_pct is boarded over boarded + no-shows + absent; null when none", Errors: []int{400, 403}})
	api.Get(reports, "/punctuality", trackingController.ReportPunctuality, api.Doc{Summary: "Punctuality report", Description: "Routes with their completed trips on time (started within 5 minutes of plan and no stop over 5 minutes late), start and stop delays and the stop reached latest by p90, least punctual first", Errors: []int{400, 403}})
	api.Get(reports, "/fleet-usage", trackingController.ReportFleetUsage, api.Doc{Summary: "Fleet usage report", Description: "Vehicles (group_by=vehicle, label the plate) or drivers (driver, label the name) with their completed trips, km, hours, riders boarded, days out and mean peak occupancy, most km first", Errors: []int{400, 403}})
	api.Get(reports, "/incidents", trackingController.ReportIncidents, api.Doc{Summary: "Incidents report", Description: "The alerts raised between from and to (UTC days), newest first, in GET /tracking/alerts' row shape, with counts by status and type and the mean minutes to resolve", Errors: []int{400, 403}})
	// :report is attendance|punctuality|fleet-usage|incidents; the handler answers 404 to any other.
	api.Get(reports, "/{report}/export", trackingController.ExportReport, api.Doc{Summary: "Export a report as CSV", Description: "Every row the report's filters match, as UTF-8 CSV with a BOM and headers in the request's language; numbers unformatted, dates ISO. Over 10 000 rows is 413 (D2)", Errors: []int{400, 404, 413}})
}

// registerPlanningRoutes is the planning screen per destination (TRACK-014): the operator's only — it
// reads the whole fleet and writes routes.
func registerPlanningRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	api.Get(r, "/organizations/{organization_id}/planning", trackingController.GetPlanning, api.Doc{Summary: "A destination's planning screen", Description: "Its riders (pickup point, legs, route each way), the Demanda counts and every vehicle free or busy at the planned hours, in one answer (TRACK-014 D2). An hour left out is the organization's running one, else 07:45 / 13:00", Errors: []int{404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/organizations/{organization_id}/optimization-runs", trackingController.CreateOptimizationRun, api.Doc{Summary: "Propose routes for a destination's riders without one", Description: "Queues a VROOM run over the riders still without a route in the legs served and the chosen vehicles; the same problem within 24 h answers its earlier run (cached). Poll GET /tracking/optimization-runs/{id}", Status: 202, Errors: []int{400, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Get(r, "/optimization-runs/{run_id}", trackingController.GetOptimizationRun, api.Doc{Summary: "A route proposal", Description: "queued|running, then done with its routes (vehicle, riders in order, stops with ETA, km, longest ride) and unassigned riders with their reason, or failed with error_code", Errors: []int{404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/optimization-runs/{run_id}/apply", trackingController.ApplyOptimizationRun, api.Doc{Summary: "Apply a route proposal as adjusted", Description: "One route pair per vehicle with its riders assigned from effective_from, in one transaction (TRACK-014 D4): a rider that can no longer be assigned aborts it all and is named", Status: 201, Errors: []int{409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
}

// registerAccessRoutes is app access granted where a person is managed (TRACK-032): a rider's
// family, a driver. The level is fixed by the route. An organization user manages its riders, so it
// may give their families the app, in its scope; the driver stays the operator's.
func registerAccessRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, access *controllers.AccessController) {
	api.Post(r, "/riders", access.CreateRider, api.Doc{Summary: "Create rider", Description: "A rider (student/employee) with up to 5 guardians in one request (TRACK-037 D2): an account by user_id, a person to invite by email and name, or a contact by name and phone. A refused guardian names its index in detail and nothing is created.", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Get(r, "/riders/{rider_id}/guardians", access.ListRiderGuardians, api.Doc{Summary: "A rider's family accounts", Description: "The guardian accounts that see the rider in the app (D3), each with the access notice to copy", Errors: []int{404}}, tenancyMW.RequirePermission(trackingPerms.RidersRead))
	api.Post(r, "/riders/{rider_id}/guardians", access.AddRiderGuardian, api.Doc{Summary: "Add a rider's guardian", Description: "An existing account by user_id, a person by email and name (found or created at the portal level), or a contact by name and phone with no account (TRACK-048 D3, user_id null, status contact); 409 when the email has web access or another app level (D4)", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Delete(r, "/riders/{rider_id}/guardians/{user_id}", access.RemoveRiderGuardian, api.Doc{Summary: "Take a family account off a rider", Description: "The account's portal access ends when no other rider names it", Status: 204, Errors: []int{404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	// Invitations at scale (TRACK-062): the same family grant for an organization or a selection, in
	// the caller's scope — an organization user's needs its admin role (TRACK-067).
	api.Post(r, "/invites", access.Invite, api.Doc{Summary: "Invite the families of an organization or of the selected riders", Description: "Every guardian contact without an account gets one by its email, the portal level and the access email; a contact with only a phone, or nothing, is skipped and named (D1)", Errors: []int{400, 403, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Post(r, "/invites/{user_id}/sent", access.MarkInviteSent, api.Doc{Summary: "Record that an invitation was sent", Description: "The operator sent the message by WhatsApp (D2); the invitation turns sent", Status: 204, Errors: []int{404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Post(r, "/invites/{user_id}/resend", access.ResendInvite, api.Doc{Summary: "Send an invitation again", Description: "Answers the message; an email invitation is mailed again and counts as sent now", Errors: []int{404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Get(r, "/organizations/{organization_id}/invites", access.ListOrganizationInvites, api.Doc{Summary: "An organization's invitations", Description: "The guardian accounts of the organization's riders with their invitation's channel and status (pending, sent, accepted, expired) and the message to send", Errors: []int{400, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersRead))
	api.Post(r, "/drivers", access.CreateDriver, api.Doc{Summary: "Create driver", Description: "A driver for one of the tenant's companies, with their app access in the same request (TRACK-047 D3): account.email finds or creates the account under the driver's name at the driver level. A refused account leaves no driver.", Status: 201, Errors: []int{400, 404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DriversWrite))
	api.Put(r, "/drivers/{driver_id}/account", access.SetDriverAccount, api.Doc{Summary: "Give a driver the app", Description: "Links an account at the driver level, found or created; the account it replaces loses the level when nothing else names it", Status: 201, Errors: []int{400, 404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DriversWrite))
	api.Delete(r, "/drivers/{driver_id}/account", access.ClearDriverAccount, api.Doc{Summary: "Take the app away from a driver", Status: 204, Errors: []int{404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DriversWrite))
}

// registerDriverRoutes is /api/v1/mobile/driver: the driver level only, and no capability — as the
// guardian's reads, what a driver may touch is their own trips, which each SP resolves from the account.
func registerDriverRoutes(router *gin.Engine, driver Driver) {
	g := router.Group("/api/v1/mobile/driver", driver.Auth...)
	g.Use(tenancyMW.RequireTenantRole(tenancyModels.RoleDriver))
	api.Get(g, "/today", driver.Controller.Today, api.Doc{Summary: "The driver's day", Description: "The driver's trips of each route's local today with their stops, tasks and riders. ETag is the hash of the day; sent back as If-None-Match it answers 304 while nothing changed", Errors: []int{403, 500}})
	api.Post(g, "/trips/{trip_id}/start", driver.Controller.StartTrip, api.Doc{Summary: "Start one of the driver's trips", Description: "planned → in_progress. Another driver's trip is 404", Errors: []int{400, 403, 404, 409}})
	api.Post(g, "/sync", driver.Controller.Sync, api.Doc{Summary: "Replay what the phone did offline", Description: "Up to 100 ops, applied in the order they were done (client_recorded_at) and stamped with that time. Each answers applied, replayed (already applied by an earlier sync) or rejected with its code; a rejected op sent again is rejected again with the same code. One op's refusal never stops the next", Errors: []int{400, 403, 500}})
	api.Post(g, "/proofs", driver.Controller.CreateProof, api.Doc{Summary: "Sign a drop-off photo's upload", Description: "A 5-minute PUT of size bytes of WebP (at most 300 KB) to a pending key of the tenant; task.done names the key as proof.photo_key and the sync stores the photo under the task (MOBILE-032)", Status: 201, Errors: []int{400, 403}})
	// The rider card and the incident report (TRACK-027).
	api.Get(g, "/riders", driver.Controller.FindRiders, api.Doc{Summary: "Find a rider without their card", Description: "The riders on the driver's trips in progress whose name contains q, or whose card code is q, each with their pending pickup; at most 20, by name", Errors: []int{400, 403}})
	api.Get(g, "/riders/{code}", driver.Controller.ResolveRider, api.Doc{Summary: "Resolve a scanned rider card", Description: "The rider the card's code names and their pending pickup on the driver's trip in progress, null when there is none. The code may be typed by hand: case and blanks are ignored. An unknown, rotated or another tenant's code is 404", Errors: []int{403, 404}})
	api.Post(g, "/incidents", driver.Controller.ReportIncident, api.Doc{Summary: "Report an incident on one of the driver's trips", Description: "A route alert on the trip's route and vehicle that names the trip and where the phone was; the board hears of it through the trip's change event. Severity follows the type: emergency critical, breakdown high, delay medium. Another driver's trip is 404", Status: 201, Errors: []int{400, 403, 404}})
}

// registerPortalRoutes is /api/v1/mobile/portal: the portal level only. The notices a guardian reads and
// mutes are its own, which each SP resolves from the account.
func registerPortalRoutes(router *gin.Engine, portal Portal) {
	g := router.Group("/api/v1/mobile/portal", portal.Auth...)
	g.Use(tenancyMW.RequireTenantRole(tenancyModels.RolePortal))
	api.Get(g, "/notifications", portal.Controller.ListNotifications, api.Doc{Summary: "The account's notices", Description: "Newest first, the account's own rows only. unread narrows to unread rows; unread_count is the whole badge either way", Errors: []int{400, 403, 500}})
	api.Post(g, "/notifications/read", portal.Controller.MarkNotificationsRead, api.Doc{Summary: "Mark notices read", Description: "The ids given (at most 100), or every unread row with all. Another account's id changes nothing", Status: 204, Errors: []int{400, 403, 500}})
	api.Get(g, "/notification-settings", portal.Controller.NotificationSettings, api.Doc{Summary: "Which pushes each rider mutes", Description: "One row per rider the account receives notices for; muted empty is every push on. A muted notice still lands in the feed", Errors: []int{403, 500}})
	api.Put(g, "/notification-settings", portal.Controller.SetNotificationSettings, api.Doc{Summary: "Replace what riders mute", Description: "Each rider named gets exactly the muted list sent; riders not named keep theirs. A rider the account receives no notices for is 404 and nothing is saved. Answers the whole list", Errors: []int{400, 403, 404, 500}})
}

// registerLiveRoutes are the snapshots a live screen reads on open and on every resync (TRACK-025):
// the operator's, like the trips they show, and an organization user's narrowed by the SP (TRACK-067).
// A rider's is also the guardian's map (MOBILE-010), so it hangs off the portal-permissive chain p and
// the SP resolves the scope, as the rider's status does.
func registerLiveRoutes(r, p *gin.RouterGroup, live *controllers.LiveController) {
	if live == nil {
		return
	}
	api.Get(r, "/live/fleet", live.GetLiveFleet, api.Doc{Summary: "Every reporting vehicle, where it is now", Description: "Each vehicle's last position with its trip in progress and route; organization_id keeps the vehicles whose trip carries one of its riders", Errors: []int{400, 403}})
	api.Get(r, "/trips/{trip_id}/live", live.GetTripLive, api.Doc{Summary: "A trip now: detail, vehicle position and ETA per pending stop", Description: "The ETA is computed on demand and cached 30 s per trip; estimate_source says whether it is valhalla, historical or fallback", Errors: []int{400, 404}})
	api.Get(p, "/riders/{rider_id}/live", live.GetRiderLive, api.Doc{Summary: "A rider's trip now: the rider's stops, the vehicle's position and the ETA to them", Description: "The guardian's map (MOBILE-010). Scoped as the rider's status; no trip in progress → trip_id null, no stops, no position", Errors: []int{400, 403, 404}})
}

// registerFleetRoutes is what the operator owns and runs: its companies, its buses, its drivers.
func registerFleetRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Companies
	api.Get(r, "/companies", trackingController.ListCompanies, api.Doc{Summary: "List transport companies", Description: "One page of the current tenant's companies, newest first", Errors: []int{400, 500}}, denyOrganization)
	api.Get(r, "/companies/{company_id}", trackingController.GetCompany, api.Doc{Summary: "Get company by ID", Description: "Get a single transport company by ID", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Post(r, "/companies", trackingController.CreateCompany, api.Doc{Summary: "Create transport company", Description: "Create a new transport company (school, corporate, transport provider)", Status: 201, Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.CompaniesWrite))
	api.Patch(r, "/companies/{company_id}", trackingController.UpdateCompany, api.Doc{Summary: "Update transport company", Description: "Update existing transport company details", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.CompaniesWrite))
	api.Delete(r, "/companies/{company_id}", trackingController.DeleteCompany, api.Doc{Summary: "Delete transport company", Description: "Delete a transport company (owner only)", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.CompaniesDelete))

	// Vehicles
	api.Get(r, "/vehicles", trackingController.ListVehicles, api.Doc{Summary: "List vehicles", Description: "One page of the current tenant's vehicles, newest first. Without company_id the whole fleet is listed; every row carries its company name.", Errors: []int{400, 500}}, denyOrganization)
	api.Get(r, "/vehicles/{vehicle_id}", trackingController.GetVehicle, api.Doc{Summary: "Get vehicle by ID", Description: "Get a single vehicle by ID", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Get(r, "/vehicles/{vehicle_id}/location", trackingController.GetVehicleCurrentLocation, api.Doc{Summary: "Get vehicle current location", Description: "Retrieve the most recent GPS location for a vehicle", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Post(r, "/vehicles", trackingController.CreateVehicle, api.Doc{Summary: "Create vehicle", Description: "Create a new vehicle for a transport company", Status: 201, Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
	api.Patch(r, "/vehicles/{vehicle_id}", trackingController.UpdateVehicle, api.Doc{Summary: "Update vehicle", Description: "Update existing vehicle details", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
	api.Delete(r, "/vehicles/{vehicle_id}", trackingController.DeleteVehicle, api.Doc{Summary: "Delete vehicle", Description: "Delete a vehicle (owner only)", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesDelete))
	api.Put(r, "/vehicles/{vehicle_id}/driver", trackingController.SetVehicleDriver, api.Doc{Summary: "Set a vehicle's usual driver", Description: "Who usually drives the vehicle; its routes without their own driver run with this one from today (TRACK-034 D1). A null driver_id clears it.", Errors: []int{400, 404, 422}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
	api.Post(r, "/vehicles/{vehicle_id}/company", trackingController.MoveVehicleCompany, api.Doc{Summary: "Move a vehicle to another company", Description: "The vehicle changes carrier; move_routes takes its routes and their pairs with it, false leaves them without a vehicle. A usual driver of another carrier is dropped (TRACK-070 D4).", Errors: []int{400, 404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.VehiclesWrite))
	api.Get(r, "/vehicles/{vehicle_id}/conflicts", trackingController.VehicleConflicts, api.Doc{Summary: "Schedule clashes of an assignment", Description: "The runs that would overlap if route_id ran on the vehicle — or, without it, the vehicle's own routes with driver_id driving them (TRACK-034 D2). Nothing is refused: the screen warns and lets the operator assign anyway.", Errors: []int{400, 404}}, denyOrganization)

	// Drivers — the carrier's staff, so the operator's alone, like the rest of the fleet. POST is
	// registerAccessRoutes': it may invite the driver to the app (TRACK-047).
	api.Get(r, "/drivers", trackingController.ListDrivers, api.Doc{Summary: "List drivers", Description: "One page of the current tenant's drivers, by surname", Errors: []int{400, 500}}, denyOrganization)
	api.Get(r, "/drivers/{driver_id}", trackingController.GetDriver, api.Doc{Summary: "Get driver by ID", Description: "Get a single driver by ID", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Patch(r, "/drivers/{driver_id}", trackingController.UpdateDriver, api.Doc{Summary: "Update driver", Description: "Update a driver; every field is optional and the carrier is immutable", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DriversWrite))
	api.Delete(r, "/drivers/{driver_id}", trackingController.DeleteDriver, api.Doc{Summary: "Delete driver", Description: "Delete a driver; refused while a route still names them as its default driver", Status: 204, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DriversDelete))
}

// registerRouteRoutes covers the board itself — the routes, their stops, and the alerts reported
// against them. Boarding and arrival are trip task transitions now (TRACK-008 D3).
func registerRouteRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Real-time operations
	api.Post(r, "/alerts", trackingController.CreateRouteAlert, api.Doc{Summary: "Create route alert", Description: "Create a delay, breakdown, or cancellation alert for a route, or for the whole operation without route_id", Status: 201, Errors: []int{400, 401, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AlertsWrite))
	// Reading and resolving them is the operator's too (TRACK-004 D3).
	api.Get(r, "/alerts", trackingController.ListRouteAlerts, api.Doc{Summary: "List alerts", Description: "Newest first. status defaults to active; all reads every status; search matches the route, the plate or the driver. Every row carries the route name (null for an alert to the whole operation), the plate, who raised and resolved it, the route's client, the driver and how many accounts were notified; trip_id and lat/lng are set on a driver's incident and the two server kinds (device_silent, off_route). summary counts active, resolved today and notified today over every filter but status", Errors: []int{400, 500}}, denyOrganization)
	api.Patch(r, "/alerts/{alert_id}/resolve", trackingController.ResolveRouteAlert, api.Doc{Summary: "Resolve an alert", Description: "An active alert only; the signed-in user is recorded as resolved_by. Answers the row as the list does", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AlertsWrite))
	api.Get(r, "/alert-settings", trackingController.AlertSettings, api.Doc{Summary: "The alert thresholds", Description: "When the server raises device_silent (minutes without a position during a trip) and off_route (metres from the planned line, for how many fixes in a row); the defaults 5 / 150 / 3 until the tenant saves its own (TRACK-071 D1)", Errors: []int{500}}, denyOrganization)
	api.Put(r, "/alert-settings", trackingController.SaveAlertSettings, api.Doc{Summary: "Save the alert thresholds", Description: "device_silent_min 1–60, off_route_m 50–1000, off_route_fixes 2–10; answers the stored row", Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AlertsWrite))

	// Routes
	api.Get(r, "/routes", trackingController.ListRoutes, api.Doc{Summary: "List routes", Description: "One page of the tenant's routes by name, with company name, plate and driver name (TRACK-002 D2)", Errors: []int{400, 500}})
	api.Get(r, "/routes/{route_id}", trackingController.GetRoute, api.Doc{Summary: "Get route by ID", Description: "Get a single route by ID", Errors: []int{400, 404, 500}})
	api.Get(r, "/routes/{route_id}/status", trackingController.GetRouteRealtimeStatus, api.Doc{Summary: "Get route real-time status", Description: "Get comprehensive route status: vehicle location, rider counts, alerts", Errors: []int{400, 401, 404, 500}})
	api.Get(r, "/routes/{route_id}/stops", trackingController.ListRouteStops, api.Doc{Summary: "List the stops a route runs on a date", Description: "The ordered stops of the route version in force on `date` — today when it is unset (TRACK-007 D1)", Errors: []int{400, 403, 404, 500}})
	// The planned line (TRACK-028): read as the stops are, scoped the same way by its SP.
	api.Get(r, "/routes/{route_id}/path", trackingController.GetRoutePath, api.Doc{Summary: "A route's line by streets", Description: "Through the stops of the route version in force on date, the route's local today when unset (TRACK-028). path is null while no line is stored. ETag is the version and when its line was stored; sent back as If-None-Match it answers 304", Errors: []int{400, 403, 404, 500}})
	api.Post(r, "/routes", trackingController.CreateRoute, api.Doc{Summary: "Create route", Description: "Create a new transportation route", Status: 201, Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Post(r, "/routes/{route_id}/return", trackingController.CreateRouteReturn, api.Doc{Summary: "Create the return of a route", Description: "Gives an outbound without a pair its return: stops reversed, same vehicle and weekdays, its riders copied (TRACK-070 D2)", Status: 201, Errors: []int{400, 404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Patch(r, "/routes/{route_id}", trackingController.UpdateRoute, api.Doc{Summary: "Update route", Description: "Update existing route details", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/routes/{route_id}", trackingController.DeleteRoute, api.Doc{Summary: "Delete route", Description: "Delete a route (owner only)", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesDelete))

	// Route versions — the history of what a route's stop list was, and the editor that changes it.
	api.Get(r, "/routes/{route_id}/versions", trackingController.ListRouteVersions, api.Doc{Summary: "List a route's stop lists over time", Description: "Every version of the route's stop list, newest first, each with how many stops it holds", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/routes/{route_id}/versions", trackingController.CreateRouteVersion, api.Doc{Summary: "Publish a route's next stop list", Description: "Publishes a new version from `effective_from`, closing the one in force the day before", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Put(r, "/routes/{route_id}/versions/{version_id}/stops", trackingController.ReplaceRouteVersionStops, api.Doc{Summary: "Replace a route version's whole stop list", Description: "One request replaces the list, so a reorder never renumbers rows one at a time; refused once the version's first day has passed", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
}

// registerTripRoutes covers the day's trips (TRACK-020): the operations board reads them like the
// route status, moving one is recording what happened (events:write), and overriding its vehicle,
// driver or start is editing the plan (routes:write). All of it is the operator's.
func registerTripRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// An organization user reads its own trips: the SP scopes the board (MOBILE-020).
	api.Get(r, "/trips", trackingController.ListTrips, api.Doc{Summary: "List a day's trips", Description: "By planned start. date empty is each route's own today; organization_id narrows to the trips carrying one of its riders. Every row carries the route, plate and driver names, stops_count, tasks_total (not cancelled) and tasks_done (done or no_show)", Errors: []int{400, 500}})
	api.Get(r, "/nav-counts", trackingController.NavCounts, api.Doc{Summary: "The taypi go rail's badges", Description: "Today's trips in progress, the active alerts and the documents expired or due within 30 days, in one read (TRACK-081); an organization user gets only its organizations' trips"}, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Get(r, "/organization/absent", trackingController.OrganizationAbsent, api.Doc{Summary: "Who a school's or company's day counts as absent", Description: "MOBILE-041: the riders behind the day's absent total, by name, in the same scope as /organization/today: an organization user reads its own organizations, staff names one with organization_id. Scoped by the SP.", Errors: []int{400, 403, 404}})
	api.Get(r, "/organization/today", trackingController.OrganizationToday, api.Doc{Summary: "A school's or company's day", Description: "MOBILE-039 D3: the client's riders on the day's trips — expected, arrived, on the way, absent, late to their shift or on time — its routes with expected arrival, and the shifts (the local planned arrival of its trips in, D2), narrowed by shift; an organization user reads its own organizations, staff names one with organization_id. Scoped by the SP, as /trips is: no permission an organization user lacks.", Errors: []int{400, 403, 404}})
	api.Get(r, "/overview", trackingController.TrackingOverview, api.Doc{Summary: "The tracking home", Description: "The day's trips in progress, planned and completed, the riders absent, the ten most late trips with their route, destination and minutes, and whether any route exists (TRACK-041 D2); an organization user's numbers are their organizations'"}, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Get(r, "/setup", trackingController.TrackingSetup, api.Doc{Summary: "The tracking home's setup assistant", Description: "Whether the tenant has any company, driver, vehicle, route and rider yet, in the order the first-run assistant asks for them (TRACK-050 D2)"}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Get(r, "/trips/{trip_id}", trackingController.GetTrip, api.Doc{Summary: "Get a trip with its stops and tasks", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Get(r, "/trips/{trip_id}/status", trackingController.GetTripStatus, api.Doc{Summary: "Get a trip's live status", Description: "Rider counts, the next pending stop, the seconds it has been due, and the vehicle's last position", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Get(r, "/trips/{trip_id}/events", trackingController.ListTripEvents, api.Doc{Summary: "A trip's event log", Description: "Oldest first: every transition, override and incident, with what it carried and who made it", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Patch(r, "/trips/{trip_id}", trackingController.UpdateTrip, api.Doc{Summary: "Override a trip", Description: "Vehicle, driver or planned start of a trip not ended; a field left out keeps its value. The trip becomes is_overridden and the materialiser keeps its header", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	// :action is start|complete|cancel, arrive|skip and done|no-show; the handler answers 404 to any other.
	api.Post(r, "/trips/{trip_id}/{action}", trackingController.TransitionTrip, api.Doc{Summary: "Start, complete or cancel a trip", Description: "start: planned → in_progress · complete: in_progress → completed · cancel: planned or in_progress → cancelled, its pending tasks cancelled and the trip is_overridden", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.EventsWrite))
	api.Post(r, "/trip-stops/{stop_id}/{action}", trackingController.TransitionTripStop, api.Doc{Summary: "Arrive at or skip a stop", Description: "Of an in-progress trip. skip turns the stop's pending pickups into no_show and its pending dropoffs into cancelled", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.EventsWrite))
	api.Post(r, "/trip-stop-tasks/{task_id}/{action}", trackingController.TransitionTripTask, api.Doc{Summary: "Mark a pickup or dropoff done, or the rider a no-show", Description: "Of an in-progress trip, at a stop not skipped. Idempotent on client_op_id: the same id again answers 200 and changes nothing; an id already used on another task is 409", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.EventsWrite))
	// Share links (TRACK-060): the operator shares any trip it may read.
	api.Post(r, "/trips/{trip_id}/share-links", trackingController.CreateTripShareLink, api.Doc{Summary: "Share a trip", Description: "A public link to a planned or running trip: route, plate, stops and the bus, never a name. ttl_minutes defaults to the trip's planned end + 60 minutes, at most 1440. The body is optional.", Status: 201, Errors: []int{404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Get(r, "/trips/{trip_id}/share-links", trackingController.ListTripShareLinks, api.Doc{Summary: "List a trip's open share links", Description: "Newest first, those neither expired nor revoked, with who made them.", Errors: []int{404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
}

// registerStopPlaceRoutes covers the places routes call at (TRACK-007). A stop place is part of the
// board the operator plans, so it rides on the routes permissions rather than a family of its own.
func registerStopPlaceRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	api.Get(r, "/stop-places", trackingController.ListStopPlaces, api.Doc{Summary: "List stop places", Description: "One page of the tenant's stop places by name, or — with `near` — the ones within `radius_m` of a point, nearest first and each carrying `distance_m`", Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Get(r, "/stop-places/{stop_place_id}", trackingController.GetStopPlace, api.Doc{Summary: "Get stop place by ID", Description: "Get a single stop place by ID", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/stop-places", trackingController.CreateStopPlace, api.Doc{Summary: "Create stop place", Description: "Create a place routes can call at; one row shared by every route that stops there (TRACK-007 D1)", Status: 201, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Patch(r, "/stop-places/{stop_place_id}", trackingController.UpdateStopPlace, api.Doc{Summary: "Update stop place", Description: "Edit a stop place; every field is optional and the coordinate moves only when both halves are sent", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/stop-places/{stop_place_id}", trackingController.DeleteStopPlace, api.Doc{Summary: "Delete stop place", Description: "Delete a stop place; refused while any route version still names it", Status: 204, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesDelete))
}

// registerScheduleRoutes covers when a route runs and what happens differently on one day
// (TRACK-018 D2). Planning the board is the operator's job, so these ride on the routes permissions
// rather than a family of their own.
func registerScheduleRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	api.Get(r, "/routes/{route_id}/schedules", trackingController.ListRouteSchedules, api.Doc{Summary: "List a route's schedules", Description: "When the route runs: a weekday bitmask (bit 0 Monday) and a departure time, over a range of dates", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/routes/{route_id}/schedules", trackingController.CreateRouteSchedule, api.Doc{Summary: "Add a recurrence to a route", Description: "days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; start_time is HH:MM", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Patch(r, "/routes/{route_id}/schedules/{schedule_id}", trackingController.UpdateRouteSchedule, api.Doc{Summary: "Edit a schedule in place", Description: "Fixes a schedule that was entered wrong; changing one from a date onward is the split endpoint", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/routes/{route_id}/schedules/{schedule_id}", trackingController.DeleteRouteSchedule, api.Doc{Summary: "Remove a recurrence from a route", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	// The split is a write of two schedules, not a delete of one: it keeps the routes-write
	// permission rather than routes-delete.
	api.Post(r, "/routes/{route_id}/schedules/{schedule_id}/split", trackingController.SplitRouteSchedule, api.Doc{Summary: "Change a schedule from a date onward", Description: "Closes the schedule the day before from_date and opens a new one carrying the changes, so the days before keep the plan they had", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
}

// registerCalendarRoutes covers the holidays and shutdowns a schedule reads (TRACK-018).
func registerCalendarRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	api.Get(r, "/calendars", trackingController.ListCalendars, api.Doc{Summary: "List the tenant's calendars", Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/calendars", trackingController.CreateCalendar, api.Doc{Summary: "Create a calendar", Description: "A named set of dates a schedule points at — a school year, the public holidays", Status: 201, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Get(r, "/calendars/{calendar_id}", trackingController.GetCalendar, api.Doc{Summary: "Get a calendar", Description: "One calendar with how many dates it holds", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Patch(r, "/calendars/{calendar_id}", trackingController.UpdateCalendar, api.Doc{Summary: "Edit a calendar", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/calendars/{calendar_id}", trackingController.DeleteCalendar, api.Doc{Summary: "Delete a calendar", Description: "Refused while a schedule still points at it", Status: 204, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesDelete))
	api.Get(r, "/calendars/{calendar_id}/dates", trackingController.ListCalendarDates, api.Doc{Summary: "List a calendar's dates", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Put(r, "/calendars/{calendar_id}/dates", trackingController.ReplaceCalendarDates, api.Doc{Summary: "Replace a calendar's whole date list", Description: "One request is the calendar, so a school year is saved once rather than one holiday at a time", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Get(r, "/calendar-dates", trackingController.ListScopeCalendarDates, api.Doc{Summary: "The tenant's and its clients' days off in a window", Description: "Dates of the tenant calendar (every route) and the client calendars (that client's routes), oldest first (TRACK-076)", Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/calendar-dates", trackingController.AddScopeCalendarDates, api.Doc{Summary: "Mark days off for every route or one client's", Description: "Creates the scope's calendar on its first date; a window wider than 92 days is refused", Status: 201, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/calendar-dates/{calendar_id}/{date}", trackingController.DeleteScopeCalendarDate, api.Doc{Summary: "Give a day off back to the routes", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
}

// registerExceptionRoutes covers the days that do not follow the recurrence, and the read-only
// preview that puts schedules, calendar, version and exceptions together (TRACK-018).
func registerExceptionRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	api.Get(r, "/routes/{route_id}/exceptions", trackingController.ListRouteExceptions, api.Doc{Summary: "List a route's exceptions", Description: "What happens differently on one day or a few; an exception overlapping the window is part of the answer", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
	api.Post(r, "/routes/{route_id}/exceptions", trackingController.CreateRouteException, api.Doc{Summary: "Record what happens differently over a range of dates", Description: "kind is one of cancel, change_time, change_vehicle, change_driver, skip_stop, add_stop, detour; payload carries exactly the keys that kind defines", Status: 201, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Delete(r, "/routes/{route_id}/exceptions/{exception_id}", trackingController.DeleteRouteException, api.Doc{Summary: "Take an exception back", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesWrite))
	api.Get(r, "/routes/{route_id}/preview", trackingController.PreviewRoute, api.Doc{Summary: "The plan a route runs over a window", Description: "Schedules, calendar, stop-list version and exceptions resolved into one row per departure; writes nothing, and a window wider than 92 days is refused", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RoutesRead))
}

// registerOrganizationRoutes covers the schools and employers, and who belongs to them.
func registerOrganizationRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController, access *controllers.AccessController) {
	// Organizations. An organization user reads its own and its admin edits it (TRACK-067 D1): the
	// SPs scope and role-check; creating and deleting one stay the operator's.
	api.Get(r, "/organizations", trackingController.ListOrganizations, api.Doc{Summary: "List organizations", Description: "One page of the current tenant's organizations, alphabetically", Errors: []int{400, 500}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsRead))
	api.Get(r, "/organizations/{organization_id}", trackingController.GetOrganization, api.Doc{Summary: "Get organization by ID", Description: "Get a single organization by ID", Errors: []int{400, 404, 500}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsRead))
	api.Post(r, "/organizations", trackingController.CreateOrganization, api.Doc{Summary: "Create organization", Description: "Create a school or employer whose people ride", Status: 201, Errors: []int{400, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite))
	api.Patch(r, "/organizations/{organization_id}", trackingController.UpdateOrganization, api.Doc{Summary: "Update organization", Description: "Update an organization; every field is optional", Errors: []int{400, 404, 500}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite))
	api.Delete(r, "/organizations/{organization_id}", trackingController.DeleteOrganization, api.Doc{Summary: "Delete organization", Description: "Delete an organization; refused while it still has riders", Status: 204, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.OrganizationsDelete))

	// Organization members. Every write goes through the access grant (TRACK-032), so a member is
	// always an account at the organization level; an organization's admin manages its own (TRACK-067).
	api.Get(r, "/organizations/{organization_id}/members", trackingController.ListOrganizationMembers, api.Doc{Summary: "List organization members", Description: "The tenant users on an organization, with their identity and role", Errors: []int{400, 404, 500}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsRead))
	api.Post(r, "/organizations/{organization_id}/members", access.InviteOrganizationMember, api.Doc{Summary: "Add an organization member, found or created", Description: "The account gets the organization level; 409 when the email has web access or another app level (D4)", Status: 201, Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite))
	api.Put(r, "/organizations/{organization_id}/members/{user_id}", access.UpsertOrganizationMember, api.Doc{Summary: "Change an organization member's role", Description: "The same grant as POST with the account named by the path, so it holds to the organization level as well", Errors: []int{400, 404, 409}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite))
	api.Delete(r, "/organizations/{organization_id}/members/{user_id}", access.RemoveOrganizationMember, api.Doc{Summary: "Remove an organization member", Description: "The account's organization level ends when nothing else names it", Status: 204, Errors: []int{404}}, tenancyMW.RequirePermission(trackingPerms.OrganizationsWrite))
}

// registerDocumentRoutes covers compliance documents and the policy behind them.
func registerDocumentRoutes(r *gin.RouterGroup, denyOrganization gin.HandlerFunc, trackingController *controllers.TrackingController) {
	// Compliance documents and the policy behind them (TRACK-016). Reading them needs no
	// permission of its own — anyone who may see the fleet may see what it must carry; writing
	// and withdrawing share one permission.
	api.Get(r, "/document-types", trackingController.ListDocumentTypes, api.Doc{Summary: "List document types", Description: "The tenant's whole compliance policy, alphabetically", Errors: []int{400, 500}}, denyOrganization)
	api.Post(r, "/document-types", trackingController.CreateDocumentType, api.Doc{Summary: "Create document type", Description: "Add a line to the tenant's compliance policy", Status: 201, Errors: []int{400, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))
	api.Patch(r, "/document-types/{document_type_id}", trackingController.UpdateDocumentType, api.Doc{Summary: "Update document type", Description: "Edit a policy line; every field is optional", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))

	api.Get(r, "/documents", trackingController.ListDocuments, api.Doc{Summary: "List documents", Description: "One page of the tenant's compliance documents, soonest expiry first", Errors: []int{400, 500}}, denyOrganization)
	api.Get(r, "/documents/{document_id}", trackingController.GetDocument, api.Doc{Summary: "Get document by ID", Description: "Get a single compliance document by ID", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Post(r, "/documents", trackingController.CreateDocument, api.Doc{Summary: "Create document", Description: "File a compliance document against one vehicle or driver", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))
	api.Patch(r, "/documents/{document_id}", trackingController.UpdateDocument, api.Doc{Summary: "Update document", Description: "Edit a document; the subject and the type are set once", Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))
	api.Delete(r, "/documents/{document_id}", trackingController.DeleteDocument, api.Doc{Summary: "Delete document", Description: "Withdraw a compliance document; its stored file goes with it", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))
	// The scan itself. Reading it is a read of the document; replacing it is a write of one.
	api.Get(r, "/documents/{document_id}/file", trackingController.DownloadDocumentFile, api.Doc{Summary: "Download a document's file", Description: "A 60-second signed link to the stored scan, issued behind auth; the scan is never public", Errors: []int{400, 404, 500}}, denyOrganization)
	api.Put(r, "/documents/{document_id}/file", trackingController.UploadDocumentFile, api.Doc{Summary: "Attach a document's file", Description: "Claim a scan uploaded on a ticket from POST /storage/uploads; PDF, JPG or PNG within the size limit", Errors: []int{400, 404, 422, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.DocumentsWrite))
}

// registerRiderRoutes is the only area a guardian reaches, and only through p.
func registerRiderRoutes(
	r *gin.RouterGroup,
	p *gin.RouterGroup,
	denyOrganization gin.HandlerFunc,
	trackingController *controllers.TrackingController,
	riderReads *controllers.RiderReadsController,
) {
	// Riders. The list and a rider's status are the guardian's two reads (TRACK-017 D3), so they
	// hang off the portal-permissive chain; the scope a guardian sees is resolved in the SP, not
	// here. Everything else about a rider stays operator- and organization-only.
	api.Get(p, "/riders", trackingController.ListRiders, api.Doc{Summary: "List riders", Description: "One page of the current tenant's riders, alphabetically", Errors: []int{400, 500}})
	// The guardian home (MOBILE-026 D1): every rider's status in one call.
	api.Get(p, "/riders/today", trackingController.ListGuardianRiderStatus, api.Doc{Summary: "Today's status of each of the guardian's riders", Description: "The guardian home (MOBILE-026 D1): every rider the signed-in guardian is linked to, each with the status GET /tracking/riders/{rider_id}/status answers — one call for all of them.", Errors: []int{403}})
	api.Get(p, "/riders/{rider_id}/status", trackingController.GetRiderStatus, api.Doc{Summary: "Get rider status (for guardians)", Description: "Get rider status including bus location, last event, and alerts (parent view)", Errors: []int{400, 401, 404, 500}})
	// The rider's day, history and absence cut-offs (TRACK-061), scoped in the SP as the status.
	if riderReads != nil {
		api.Get(p, "/riders/{rider_id}/today", riderReads.RiderToday, api.Doc{Summary: "The rider's day", Description: "Every trip of each route's local today carrying the rider, with the pickup and dropoff and the ETA on the trip in progress; today's absence and the carrier's phone. ETag is the hash of the body; sent back as If-None-Match it answers 304", Errors: []int{400, 403, 404}})
		api.Get(p, "/riders/{rider_id}/history", riderReads.RiderHistory, api.Doc{Summary: "The rider's past days", Description: "One row per day, newest first, with a status (rode, partial, no_show, absent, cancelled), a leg per trip with pickup and dropoff times, and the absence; days with neither are left out. Default the 30 days up to the rider's local yesterday, at most 31", Errors: []int{400, 403, 404}})
		api.Get(p, "/riders/{rider_id}/absence-cutoffs", riderReads.RiderAbsenceCutoffs, api.Doc{Summary: "Until when each day's absence can be reported", Description: "Per day: whether the rider has a pickup, the guardian's cut-off before it and whether it has passed — the rule POST /absences answers 409 cutoff on. Default today and the next 13 days, at most 31", Errors: []int{400, 403, 404}})
	}
	// A drop-off's proof photo (MOBILE-032): staff, an organization for its riders and a guardian for
	// their own — one route, since r and p share the path; the SP scopes it as the status above.
	api.Get(p, "/trip-stop-tasks/{task_id}/proof", trackingController.GetTripTaskProof, api.Doc{Summary: "Link to a drop-off's proof photo", Description: "A 60-second signed link to the photo the driver took at the drop-off (MOBILE-032). An organization user reaches its riders' tasks, a guardian their own riders'; no photo, or a task out of scope, is 404", Errors: []int{400, 404}})
	// Absences (TRACK-022) are a guardian's too: the SPs resolve the scope and mark a guardian's write
	// `portal`. A staff caller still needs riders:write to change them.
	api.Get(p, "/riders/{rider_id}/absences", trackingController.ListRiderAbsences, api.Doc{Summary: "List a rider's absences", Description: "Oldest first, those overlapping [from, to]. A guardian reads only their own riders.", Errors: []int{400, 404}})
	api.Post(p, "/riders/{rider_id}/absences", trackingController.CreateRiderAbsence, api.Doc{Summary: "Report a rider's absence", Description: "Cancels the rider's pending tasks on the range's trips that have not ended. A guardian may report only before the organization's cut-off.", Status: 201, Errors: []int{400, 404, 409}}, guardianOrPermission(trackingPerms.RidersWrite))
	api.Post(p, "/riders/{rider_id}/absences/batch", trackingController.CreateRiderAbsencesBatch, api.Doc{Summary: "Report a rider's absence on several runs of days", Description: "1–31 ranges in one transaction, as POST /absences each: all or nothing — a cut-off, overlap or scope refusal on any range creates none and names that range's first day as detail.date (MOBILE-038 D2).", Status: 201, Errors: []int{400, 404, 409}}, guardianOrPermission(trackingPerms.RidersWrite))
	api.Delete(p, "/riders/{rider_id}/absences/{absence_id}", trackingController.DeleteRiderAbsence, api.Doc{Summary: "Delete a rider's absence", Description: "The rider is seated again on the planned trips the materialiser still owns. A guardian may delete only before the organization's cut-off.", Status: 204, Errors: []int{404, 409}}, guardianOrPermission(trackingPerms.RidersWrite))
	// A guardian shares its rider's trip of today and revokes what it shared (TRACK-060); the SPs scope it.
	api.Post(p, "/riders/{rider_id}/share-links", trackingController.CreateRiderShareLink, api.Doc{Summary: "Share a rider's trip of today", Description: "A guardian's public link to their rider's trip of today still to run: that rider's stops and first name only. 409 when there is no such trip. The body is optional.", Status: 201, Errors: []int{404, 409}}, guardianOrPermission(trackingPerms.RoutesRead))
	api.Delete(p, "/share-links/{link_id}", trackingController.RevokeShareLink, api.Doc{Summary: "Revoke a share link", Description: "The link answers 404 from now on. A guardian revokes only the links they made.", Status: 204, Errors: []int{404}}, guardianOrPermission(trackingPerms.RoutesRead))
	api.Get(r, "/riders/{rider_id}/suggestions", trackingController.SuggestRiderStops, api.Doc{Summary: "Suggest stops near a rider's home", Description: "Stops within max_walk_m (straight line) of the rider's home on the current version of an active route of the direction, nearest first then most free seats.", Errors: []int{400, 404, 409}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsRead))
	api.Get(r, "/riders/groups", trackingController.ListRiderGroups, api.Doc{Summary: "The groups riders use", Description: "The distinct groups (\"5to B\") of the tenant's riders, optionally of one organization (TRACK-039 D2)"})
	// The printable cards of a route, a client or a rider (TRACK-072 D2), scoped as the list is.
	api.Get(r, "/riders/cards", trackingController.ListRiderCards, api.Doc{Summary: "Printable rider cards", Description: "The cards of one route's riders (its pair's too, in boarding order), of one client, or of one rider: name, client, the outbound route and pickup stop, and the code a driver scans. Exactly one filter. An organization user reaches only their organizations' riders", Errors: []int{400}})
	// Saved places (TRACK-044 D4): read and changed as the rider is.
	api.Get(r, "/riders/{rider_id}/places", trackingController.ListRiderPlaces, api.Doc{Summary: "List a rider's saved places", Description: "The default first: it is the rider's home.", Errors: []int{400, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersRead))
	api.Post(r, "/riders/{rider_id}/places", trackingController.CreateRiderPlace, api.Doc{Summary: "Save a place for a rider", Description: "The rider's first place, or one sent with is_default, becomes the rider's home.", Status: 201, Errors: []int{400, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Patch(r, "/riders/{rider_id}/places/{place_id}", trackingController.UpdateRiderPlace, api.Doc{Summary: "Edit a rider's place", Description: "A field left out keeps its value; is_default true makes the place the rider's home.", Errors: []int{400, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Delete(r, "/riders/{rider_id}/places/{place_id}", trackingController.DeleteRiderPlace, api.Doc{Summary: "Delete a rider's place", Description: "The default place goes only when it is the last one, and takes the home pin with it.", Status: 204, Errors: []int{404, 409}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	// Who outside the operator read the rider (TRACK-063 D2): the operator's own staff only.
	api.Get(r, "/riders/{rider_id}/audit", trackingController.ListRiderAccessLog, api.Doc{Summary: "Who outside the operator read a rider", Description: "Guardians' and organization users' reads of the rider's status, home, live map and absences, one entry per reader, action and hour, newest first (TRACK-063). Staff only.", Errors: []int{400, 404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.RidersRead))
	api.Get(r, "/riders/{rider_id}", trackingController.GetRider, api.Doc{Summary: "Get rider by ID", Description: "Get a single rider by ID", Errors: []int{400, 404, 500}})
	// The card's code (TRACK-027): read as the rider is, rotated as a rider is changed.
	api.Get(r, "/riders/{rider_id}/qr-token", trackingController.RiderQRToken, api.Doc{Summary: "Read or rotate a rider's card code", Description: "The code printed on the rider's card, which a driver scans. POST replaces it, so the old card resolves nothing (riders:write). An organization user reaches only their organizations' riders", Errors: []int{400, 404}})
	api.Post(r, "/riders/{rider_id}/qr-token", trackingController.RiderQRToken, api.Doc{Summary: "Read or rotate a rider's card code", Description: "The code printed on the rider's card, which a driver scans. POST replaces it, so the old card resolves nothing (riders:write). An organization user reaches only their organizations' riders", Errors: []int{400, 404}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Patch(r, "/riders/{rider_id}", trackingController.UpdateRider, api.Doc{Summary: "Update rider", Description: "Update existing rider details", Errors: []int{400, 404, 500}}, tenancyMW.RequirePermission(trackingPerms.RidersWrite))
	api.Delete(r, "/riders/{rider_id}", trackingController.DeleteRider, api.Doc{Summary: "Delete rider", Description: "Delete a rider (owner only)", Status: 204, Errors: []int{400, 404, 500}}, tenancyMW.RequirePermission(trackingPerms.RidersDelete))

	// Assignments (TRACK-009)
	api.Get(r, "/assignments", trackingController.ListAssignments, api.Doc{Summary: "List rider route assignments", Description: "Every row carries the rider, route and stop names and the route's direction; date narrows to the assignments in force that day", Errors: []int{400, 500}}, tenancyMW.RequirePermission(trackingPerms.AssignmentsRead))
	api.Post(r, "/assignments", trackingController.CreateAssignment, api.Doc{Summary: "Assign a rider to a route", Description: "days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; valid_until empty is open-ended. Answers the route's capacity warnings, which never block", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsManage))
	api.Post(r, "/assignments/bulk", trackingController.CreateAssignmentsBulk, api.Doc{Summary: "Assign many riders at once", Description: "All or none: the first item refused rolls back every one and the error's detail names its zero-based index", Status: 201, Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsManage))
	api.Post(r, "/routes/{route_id}/assignments/bulk", trackingController.BulkAssignRiders, api.Doc{Summary: "Add many riders to a route", Description: "Up to 100 riders in one call (TRACK-039 D1), each with its home as its stop and on the return too when the route has one; a rider without a home pin or already on another route that day is skipped with its reason and the rest still go in", Errors: []int{400, 404}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsManage))
	api.Patch(r, "/assignments/{assignment_id}", trackingController.UpdateAssignment, api.Doc{Summary: "Edit an assignment", Description: "Every field but the rider; a field left out keeps its value. Answers the route's capacity warnings", Errors: []int{400, 404, 409, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsManage))
	api.Delete(r, "/assignments/{assignment_id}", trackingController.DeleteAssignment, api.Doc{Summary: "Remove an assignment", Status: 204, Errors: []int{400, 404, 500}}, denyOrganization, tenancyMW.RequirePermission(trackingPerms.AssignmentsManage))
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
	api.Post(g, "/companies", trackingController.CreateCompany, api.Doc{Summary: "Create transport company", Description: "Create a new transport company (school, corporate, transport provider)", Status: 201, Errors: []int{400, 500}})
	api.Patch(g, "/companies/{company_id}", trackingController.UpdateCompany, api.Doc{Summary: "Update transport company", Description: "Update existing transport company details", Errors: []int{400, 404, 500}})
	api.Get(g, "/companies", trackingController.ListCompanies, api.Doc{Summary: "List transport companies", Description: "One page of the current tenant's companies, newest first", Errors: []int{400, 500}})
	api.Get(g, "/companies/{company_id}", trackingController.GetCompany, api.Doc{Summary: "Get company by ID", Description: "Get a single transport company by ID", Errors: []int{400, 404, 500}})
	api.Delete(g, "/companies/{company_id}", trackingController.DeleteCompany, api.Doc{Summary: "Delete transport company", Description: "Delete a transport company (owner only)", Status: 204, Errors: []int{400, 404, 500}})
	api.Post(g, "/vehicles", trackingController.CreateVehicle, api.Doc{Summary: "Create vehicle", Description: "Create a new vehicle for a transport company", Status: 201, Errors: []int{400, 500}})
	api.Get(g, "/vehicles", trackingController.ListVehicles, api.Doc{Summary: "List vehicles", Description: "One page of the current tenant's vehicles, newest first. Without company_id the whole fleet is listed; every row carries its company name.", Errors: []int{400, 500}})
	api.Get(g, "/vehicles/{vehicle_id}", trackingController.GetVehicle, api.Doc{Summary: "Get vehicle by ID", Description: "Get a single vehicle by ID", Errors: []int{400, 404, 500}})
	api.Get(g, "/vehicles/{vehicle_id}/location", trackingController.GetVehicleCurrentLocation, api.Doc{Summary: "Get vehicle current location", Description: "Retrieve the most recent GPS location for a vehicle", Errors: []int{400, 404, 500}})
	api.Patch(g, "/vehicles/{vehicle_id}", trackingController.UpdateVehicle, api.Doc{Summary: "Update vehicle", Description: "Update existing vehicle details", Errors: []int{400, 404, 500}})
	api.Delete(g, "/vehicles/{vehicle_id}", trackingController.DeleteVehicle, api.Doc{Summary: "Delete vehicle", Description: "Delete a vehicle (owner only)", Status: 204, Errors: []int{400, 404, 500}})
	api.Put(g, "/vehicles/{vehicle_id}/driver", trackingController.SetVehicleDriver, api.Doc{Summary: "Set a vehicle's usual driver", Description: "Who usually drives the vehicle; its routes without their own driver run with this one from today (TRACK-034 D1). A null driver_id clears it.", Errors: []int{400, 404, 422}})
	api.Post(g, "/vehicles/{vehicle_id}/company", trackingController.MoveVehicleCompany, api.Doc{Summary: "Move a vehicle to another company", Description: "The vehicle changes carrier; move_routes takes its routes and their pairs with it, false leaves them without a vehicle. A usual driver of another carrier is dropped (TRACK-070 D4).", Errors: []int{400, 404, 409}})
	api.Get(g, "/vehicles/{vehicle_id}/conflicts", trackingController.VehicleConflicts, api.Doc{Summary: "Schedule clashes of an assignment", Description: "The runs that would overlap if route_id ran on the vehicle — or, without it, the vehicle's own routes with driver_id driving them (TRACK-034 D2). Nothing is refused: the screen warns and lets the operator assign anyway.", Errors: []int{400, 404}})
	api.Post(g, "/drivers", trackingController.CreateDriver, api.Doc{Summary: "Create driver", Description: "Create a driver for one of the tenant's carriers", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/drivers", trackingController.ListDrivers, api.Doc{Summary: "List drivers", Description: "One page of the current tenant's drivers, by surname", Errors: []int{400, 500}})
	api.Get(g, "/drivers/{driver_id}", trackingController.GetDriver, api.Doc{Summary: "Get driver by ID", Description: "Get a single driver by ID", Errors: []int{400, 404, 500}})
	api.Patch(g, "/drivers/{driver_id}", trackingController.UpdateDriver, api.Doc{Summary: "Update driver", Description: "Update a driver; every field is optional and the carrier is immutable", Errors: []int{400, 404, 409, 500}})
	api.Delete(g, "/drivers/{driver_id}", trackingController.DeleteDriver, api.Doc{Summary: "Delete driver", Description: "Delete a driver; refused while a route still names them as its default driver", Status: 204, Errors: []int{400, 404, 409, 500}})
}

// registerOpenRouteRoutes mirrors registerRouteRoutes without the guards.
func registerOpenRouteRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Post(g, "/alerts", trackingController.CreateRouteAlert, api.Doc{Summary: "Create route alert", Description: "Create a delay, breakdown, or cancellation alert for a route, or for the whole operation without route_id", Status: 201, Errors: []int{400, 401, 404, 500}})
	api.Get(g, "/alerts", trackingController.ListRouteAlerts, api.Doc{Summary: "List alerts", Description: "Newest first. status defaults to active; all reads every status; search matches the route, the plate or the driver. Every row carries the route name (null for an alert to the whole operation), the plate, who raised and resolved it, the route's client, the driver and how many accounts were notified; trip_id and lat/lng are set on a driver's incident and the two server kinds (device_silent, off_route). summary counts active, resolved today and notified today over every filter but status", Errors: []int{400, 500}})
	api.Patch(g, "/alerts/{alert_id}/resolve", trackingController.ResolveRouteAlert, api.Doc{Summary: "Resolve an alert", Description: "An active alert only; the signed-in user is recorded as resolved_by. Answers the row as the list does", Errors: []int{400, 404, 409, 500}})
	api.Post(g, "/routes", trackingController.CreateRoute, api.Doc{Summary: "Create route", Description: "Create a new transportation route", Status: 201, Errors: []int{400, 500}})
	api.Get(g, "/routes", trackingController.ListRoutes, api.Doc{Summary: "List routes", Description: "One page of the tenant's routes by name, with company name, plate and driver name (TRACK-002 D2)", Errors: []int{400, 500}})
	api.Get(g, "/routes/{route_id}", trackingController.GetRoute, api.Doc{Summary: "Get route by ID", Description: "Get a single route by ID", Errors: []int{400, 404, 500}})
	api.Get(g, "/routes/{route_id}/status", trackingController.GetRouteRealtimeStatus, api.Doc{Summary: "Get route real-time status", Description: "Get comprehensive route status: vehicle location, rider counts, alerts", Errors: []int{400, 401, 404, 500}})
	api.Get(g, "/routes/{route_id}/stops", trackingController.ListRouteStops, api.Doc{Summary: "List the stops a route runs on a date", Description: "The ordered stops of the route version in force on `date` — today when it is unset (TRACK-007 D1)", Errors: []int{400, 403, 404, 500}})
	api.Get(g, "/routes/{route_id}/path", trackingController.GetRoutePath, api.Doc{Summary: "A route's line by streets", Errors: []int{400, 404, 500}})
	api.Post(g, "/routes/{route_id}/return", trackingController.CreateRouteReturn, api.Doc{Summary: "Create the return of a route", Description: "Gives an outbound without a pair its return: stops reversed, same vehicle and weekdays, its riders copied (TRACK-070 D2)", Status: 201, Errors: []int{400, 404, 409}})
	api.Patch(g, "/routes/{route_id}", trackingController.UpdateRoute, api.Doc{Summary: "Update route", Description: "Update existing route details", Errors: []int{400, 404, 500}})
	api.Delete(g, "/routes/{route_id}", trackingController.DeleteRoute, api.Doc{Summary: "Delete route", Description: "Delete a route (owner only)", Status: 204, Errors: []int{400, 404, 500}})
	api.Get(g, "/routes/{route_id}/versions", trackingController.ListRouteVersions, api.Doc{Summary: "List a route's stop lists over time", Description: "Every version of the route's stop list, newest first, each with how many stops it holds", Errors: []int{400, 404, 500}})
	api.Post(g, "/routes/{route_id}/versions", trackingController.CreateRouteVersion, api.Doc{Summary: "Publish a route's next stop list", Description: "Publishes a new version from `effective_from`, closing the one in force the day before", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Put(g, "/routes/{route_id}/versions/{version_id}/stops", trackingController.ReplaceRouteVersionStops, api.Doc{Summary: "Replace a route version's whole stop list", Description: "One request replaces the list, so a reorder never renumbers rows one at a time; refused once the version's first day has passed", Errors: []int{400, 404, 409, 500}})
}

// registerOpenStopPlaceRoutes mirrors registerStopPlaceRoutes without the guards.
func registerOpenStopPlaceRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/stop-places", trackingController.ListStopPlaces, api.Doc{Summary: "List stop places", Description: "One page of the tenant's stop places by name, or — with `near` — the ones within `radius_m` of a point, nearest first and each carrying `distance_m`", Errors: []int{400, 500}})
	api.Get(g, "/stop-places/{stop_place_id}", trackingController.GetStopPlace, api.Doc{Summary: "Get stop place by ID", Description: "Get a single stop place by ID", Errors: []int{400, 404, 500}})
	api.Post(g, "/stop-places", trackingController.CreateStopPlace, api.Doc{Summary: "Create stop place", Description: "Create a place routes can call at; one row shared by every route that stops there (TRACK-007 D1)", Status: 201, Errors: []int{400, 404, 500}})
	api.Patch(g, "/stop-places/{stop_place_id}", trackingController.UpdateStopPlace, api.Doc{Summary: "Update stop place", Description: "Edit a stop place; every field is optional and the coordinate moves only when both halves are sent", Errors: []int{400, 404, 500}})
	api.Delete(g, "/stop-places/{stop_place_id}", trackingController.DeleteStopPlace, api.Doc{Summary: "Delete stop place", Description: "Delete a stop place; refused while any route version still names it", Status: 204, Errors: []int{400, 404, 409, 500}})
}

// registerOpenScheduleRoutes mirrors registerScheduleRoutes without the guards.
func registerOpenScheduleRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/routes/{route_id}/schedules", trackingController.ListRouteSchedules, api.Doc{Summary: "List a route's schedules", Description: "When the route runs: a weekday bitmask (bit 0 Monday) and a departure time, over a range of dates", Errors: []int{400, 404, 500}})
	api.Post(g, "/routes/{route_id}/schedules", trackingController.CreateRouteSchedule, api.Doc{Summary: "Add a recurrence to a route", Description: "days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; start_time is HH:MM", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Patch(g, "/routes/{route_id}/schedules/{schedule_id}", trackingController.UpdateRouteSchedule, api.Doc{Summary: "Edit a schedule in place", Description: "Fixes a schedule that was entered wrong; changing one from a date onward is the split endpoint", Errors: []int{400, 404, 409, 500}})
	api.Delete(g, "/routes/{route_id}/schedules/{schedule_id}", trackingController.DeleteRouteSchedule, api.Doc{Summary: "Remove a recurrence from a route", Status: 204, Errors: []int{400, 404, 500}})
	api.Post(g, "/routes/{route_id}/schedules/{schedule_id}/split", trackingController.SplitRouteSchedule, api.Doc{Summary: "Change a schedule from a date onward", Description: "Closes the schedule the day before from_date and opens a new one carrying the changes, so the days before keep the plan they had", Status: 201, Errors: []int{400, 404, 409, 500}})
}

// registerOpenCalendarRoutes mirrors registerCalendarRoutes without the guards.
func registerOpenCalendarRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/calendars", trackingController.ListCalendars, api.Doc{Summary: "List the tenant's calendars", Errors: []int{400, 500}})
	api.Post(g, "/calendars", trackingController.CreateCalendar, api.Doc{Summary: "Create a calendar", Description: "A named set of dates a schedule points at — a school year, the public holidays", Status: 201, Errors: []int{400, 404, 500}})
	api.Get(g, "/calendars/{calendar_id}", trackingController.GetCalendar, api.Doc{Summary: "Get a calendar", Description: "One calendar with how many dates it holds", Errors: []int{400, 404, 500}})
	api.Patch(g, "/calendars/{calendar_id}", trackingController.UpdateCalendar, api.Doc{Summary: "Edit a calendar", Errors: []int{400, 404, 500}})
	api.Delete(g, "/calendars/{calendar_id}", trackingController.DeleteCalendar, api.Doc{Summary: "Delete a calendar", Description: "Refused while a schedule still points at it", Status: 204, Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/calendars/{calendar_id}/dates", trackingController.ListCalendarDates, api.Doc{Summary: "List a calendar's dates", Errors: []int{400, 404, 500}})
	api.Put(g, "/calendars/{calendar_id}/dates", trackingController.ReplaceCalendarDates, api.Doc{Summary: "Replace a calendar's whole date list", Description: "One request is the calendar, so a school year is saved once rather than one holiday at a time", Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/calendar-dates", trackingController.ListScopeCalendarDates, api.Doc{Summary: "The tenant's and its clients' days off in a window", Description: "Dates of the tenant calendar (every route) and the client calendars (that client's routes), oldest first (TRACK-076)", Errors: []int{400, 500}})
	api.Post(g, "/calendar-dates", trackingController.AddScopeCalendarDates, api.Doc{Summary: "Mark days off for every route or one client's", Description: "Creates the scope's calendar on its first date; a window wider than 92 days is refused", Status: 201, Errors: []int{400, 404, 500}})
	api.Delete(g, "/calendar-dates/{calendar_id}/{date}", trackingController.DeleteScopeCalendarDate, api.Doc{Summary: "Give a day off back to the routes", Status: 204, Errors: []int{400, 404, 500}})
}

// registerOpenExceptionRoutes mirrors registerExceptionRoutes without the guards.
func registerOpenExceptionRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/routes/{route_id}/exceptions", trackingController.ListRouteExceptions, api.Doc{Summary: "List a route's exceptions", Description: "What happens differently on one day or a few; an exception overlapping the window is part of the answer", Errors: []int{400, 404, 500}})
	api.Post(g, "/routes/{route_id}/exceptions", trackingController.CreateRouteException, api.Doc{Summary: "Record what happens differently over a range of dates", Description: "kind is one of cancel, change_time, change_vehicle, change_driver, skip_stop, add_stop, detour; payload carries exactly the keys that kind defines", Status: 201, Errors: []int{400, 404, 500}})
	api.Delete(g, "/routes/{route_id}/exceptions/{exception_id}", trackingController.DeleteRouteException, api.Doc{Summary: "Take an exception back", Status: 204, Errors: []int{400, 404, 500}})
	api.Get(g, "/routes/{route_id}/preview", trackingController.PreviewRoute, api.Doc{Summary: "The plan a route runs over a window", Description: "Schedules, calendar, stop-list version and exceptions resolved into one row per departure; writes nothing, and a window wider than 92 days is refused", Errors: []int{400, 404, 500}})
}

// registerOpenOrganizationRoutes mirrors registerOrganizationRoutes without the guards.
func registerOpenOrganizationRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Post(g, "/organizations", trackingController.CreateOrganization, api.Doc{Summary: "Create organization", Description: "Create a school or employer whose people ride", Status: 201, Errors: []int{400, 500}})
	api.Get(g, "/organizations", trackingController.ListOrganizations, api.Doc{Summary: "List organizations", Description: "One page of the current tenant's organizations, alphabetically", Errors: []int{400, 500}})
	api.Get(g, "/organizations/{organization_id}", trackingController.GetOrganization, api.Doc{Summary: "Get organization by ID", Description: "Get a single organization by ID", Errors: []int{400, 404, 500}})
	api.Patch(g, "/organizations/{organization_id}", trackingController.UpdateOrganization, api.Doc{Summary: "Update organization", Description: "Update an organization; every field is optional", Errors: []int{400, 404, 500}})
	api.Delete(g, "/organizations/{organization_id}", trackingController.DeleteOrganization, api.Doc{Summary: "Delete organization", Description: "Delete an organization; refused while it still has riders", Status: 204, Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/organizations/{organization_id}/members", trackingController.ListOrganizationMembers, api.Doc{Summary: "List organization members", Description: "The tenant users on an organization, with their identity and role", Errors: []int{400, 404, 500}})
	api.Put(g, "/organizations/{organization_id}/members/{user_id}", trackingController.UpsertOrganizationMember, api.Doc{Summary: "Add an organization member or change their role", Description: "Idempotent: the same call adds a member and re-roles one already there", Errors: []int{400, 404, 500}})
	api.Delete(g, "/organizations/{organization_id}/members/{user_id}", trackingController.DeleteOrganizationMember, api.Doc{Summary: "Remove an organization member", Description: "Take a user off an organization; the user account itself is untouched", Status: 204, Errors: []int{400, 404, 500}})
}

// registerOpenDocumentRoutes mirrors registerDocumentRoutes without the guards.
func registerOpenDocumentRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/document-types", trackingController.ListDocumentTypes, api.Doc{Summary: "List document types", Description: "The tenant's whole compliance policy, alphabetically", Errors: []int{400, 500}})
	api.Post(g, "/document-types", trackingController.CreateDocumentType, api.Doc{Summary: "Create document type", Description: "Add a line to the tenant's compliance policy", Status: 201, Errors: []int{400, 409, 500}})
	api.Patch(g, "/document-types/{document_type_id}", trackingController.UpdateDocumentType, api.Doc{Summary: "Update document type", Description: "Edit a policy line; every field is optional", Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/documents", trackingController.ListDocuments, api.Doc{Summary: "List documents", Description: "One page of the tenant's compliance documents, soonest expiry first", Errors: []int{400, 500}})
	api.Get(g, "/documents/{document_id}", trackingController.GetDocument, api.Doc{Summary: "Get document by ID", Description: "Get a single compliance document by ID", Errors: []int{400, 404, 500}})
	api.Post(g, "/documents", trackingController.CreateDocument, api.Doc{Summary: "Create document", Description: "File a compliance document against one vehicle or driver", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Patch(g, "/documents/{document_id}", trackingController.UpdateDocument, api.Doc{Summary: "Update document", Description: "Edit a document; the subject and the type are set once", Errors: []int{400, 404, 500}})
	api.Delete(g, "/documents/{document_id}", trackingController.DeleteDocument, api.Doc{Summary: "Delete document", Description: "Withdraw a compliance document; its stored file goes with it", Status: 204, Errors: []int{400, 404, 500}})
	api.Get(g, "/documents/{document_id}/file", trackingController.DownloadDocumentFile, api.Doc{Summary: "Download a document's file", Description: "A 60-second signed link to the stored scan, issued behind auth; the scan is never public", Errors: []int{400, 404, 500}})
	api.Put(g, "/documents/{document_id}/file", trackingController.UploadDocumentFile, api.Doc{Summary: "Attach a document's file", Errors: []int{400, 404, 422, 500}})
}

// registerOpenRiderRoutes mirrors registerRiderRoutes without the guards.
func registerOpenRiderRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Post(g, "/riders", trackingController.CreateRider, api.Doc{Summary: "Create rider", Status: 201, Errors: []int{400, 404, 409}})
	api.Get(g, "/riders", trackingController.ListRiders, api.Doc{Summary: "List riders", Description: "One page of the current tenant's riders, alphabetically", Errors: []int{400, 500}})
	api.Get(g, "/riders/{rider_id}", trackingController.GetRider, api.Doc{Summary: "Get rider by ID", Description: "Get a single rider by ID", Errors: []int{400, 404, 500}})
	api.Get(g, "/riders/{rider_id}/status", trackingController.GetRiderStatus, api.Doc{Summary: "Get rider status (for guardians)", Description: "Get rider status including bus location, last event, and alerts (parent view)", Errors: []int{400, 401, 404, 500}})
	api.Patch(g, "/riders/{rider_id}", trackingController.UpdateRider, api.Doc{Summary: "Update rider", Description: "Update existing rider details", Errors: []int{400, 404, 500}})
	api.Delete(g, "/riders/{rider_id}", trackingController.DeleteRider, api.Doc{Summary: "Delete rider", Description: "Delete a rider (owner only)", Status: 204, Errors: []int{400, 404, 500}})
	api.Get(g, "/riders/{rider_id}/absences", trackingController.ListRiderAbsences, api.Doc{Summary: "List a rider's absences", Description: "Oldest first, those overlapping [from, to]. A guardian reads only their own riders.", Errors: []int{400, 404}})
	api.Post(g, "/riders/{rider_id}/absences", trackingController.CreateRiderAbsence, api.Doc{Summary: "Report a rider's absence", Description: "Cancels the rider's pending tasks on the range's trips that have not ended. A guardian may report only before the organization's cut-off.", Status: 201, Errors: []int{400, 404, 409}})
	api.Post(g, "/riders/{rider_id}/absences/batch", trackingController.CreateRiderAbsencesBatch, api.Doc{Summary: "Report a rider's absence on several runs of days", Description: "1–31 ranges in one transaction, as POST /absences each: all or nothing — a cut-off, overlap or scope refusal on any range creates none and names that range's first day as detail.date (MOBILE-038 D2).", Status: 201, Errors: []int{400, 404, 409}})
	api.Delete(g, "/riders/{rider_id}/absences/{absence_id}", trackingController.DeleteRiderAbsence, api.Doc{Summary: "Delete a rider's absence", Description: "The rider is seated again on the planned trips the materialiser still owns. A guardian may delete only before the organization's cut-off.", Status: 204, Errors: []int{404, 409}})
	api.Get(g, "/riders/{rider_id}/suggestions", trackingController.SuggestRiderStops, api.Doc{Summary: "Suggest stops near a rider's home", Description: "Stops within max_walk_m (straight line) of the rider's home on the current version of an active route of the direction, nearest first then most free seats.", Errors: []int{400, 404, 409}})
	api.Get(g, "/riders/{rider_id}/places", trackingController.ListRiderPlaces, api.Doc{Summary: "List a rider's saved places", Description: "The default first: it is the rider's home.", Errors: []int{400, 404}})
	api.Post(g, "/riders/{rider_id}/places", trackingController.CreateRiderPlace, api.Doc{Summary: "Save a place for a rider", Description: "The rider's first place, or one sent with is_default, becomes the rider's home.", Status: 201, Errors: []int{400, 404}})
	api.Patch(g, "/riders/{rider_id}/places/{place_id}", trackingController.UpdateRiderPlace, api.Doc{Summary: "Edit a rider's place", Description: "A field left out keeps its value; is_default true makes the place the rider's home.", Errors: []int{400, 404}})
	api.Delete(g, "/riders/{rider_id}/places/{place_id}", trackingController.DeleteRiderPlace, api.Doc{Summary: "Delete a rider's place", Description: "The default place goes only when it is the last one, and takes the home pin with it.", Status: 204, Errors: []int{404, 409}})
	api.Get(g, "/assignments", trackingController.ListAssignments, api.Doc{Summary: "List rider route assignments", Description: "Every row carries the rider, route and stop names and the route's direction; date narrows to the assignments in force that day", Errors: []int{400, 500}})
	api.Post(g, "/assignments", trackingController.CreateAssignment, api.Doc{Summary: "Assign a rider to a route", Description: "days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; valid_until empty is open-ended. Answers the route's capacity warnings, which never block", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Post(g, "/assignments/bulk", trackingController.CreateAssignmentsBulk, api.Doc{Summary: "Assign many riders at once", Description: "All or none: the first item refused rolls back every one and the error's detail names its zero-based index", Status: 201, Errors: []int{400, 404, 409, 500}})
	api.Patch(g, "/assignments/{assignment_id}", trackingController.UpdateAssignment, api.Doc{Summary: "Edit an assignment", Description: "Every field but the rider; a field left out keeps its value. Answers the route's capacity warnings", Errors: []int{400, 404, 409, 500}})
	api.Delete(g, "/assignments/{assignment_id}", trackingController.DeleteAssignment, api.Doc{Summary: "Remove an assignment", Status: 204, Errors: []int{400, 404, 500}})
}

// registerOpenTripRoutes mirrors registerTripRoutes without the guards.
func registerOpenTripRoutes(g *gin.RouterGroup, trackingController *controllers.TrackingController) {
	api.Get(g, "/trips", trackingController.ListTrips, api.Doc{Summary: "List a day's trips", Description: "By planned start. date empty is each route's own today; organization_id narrows to the trips carrying one of its riders. Every row carries the route, plate and driver names, stops_count, tasks_total (not cancelled) and tasks_done (done or no_show)", Errors: []int{400, 500}})
	api.Get(g, "/trips/{trip_id}", trackingController.GetTrip, api.Doc{Summary: "Get a trip with its stops and tasks", Errors: []int{400, 404, 500}})
	api.Get(g, "/trips/{trip_id}/status", trackingController.GetTripStatus, api.Doc{Summary: "Get a trip's live status", Description: "Rider counts, the next pending stop, the seconds it has been due, and the vehicle's last position", Errors: []int{400, 404, 500}})
	api.Get(g, "/trips/{trip_id}/events", trackingController.ListTripEvents, api.Doc{Summary: "A trip's event log", Description: "Oldest first: every transition, override and incident, with what it carried and who made it", Errors: []int{400, 404, 500}})
	api.Patch(g, "/trips/{trip_id}", trackingController.UpdateTrip, api.Doc{Summary: "Override a trip", Description: "Vehicle, driver or planned start of a trip not ended; a field left out keeps its value. The trip becomes is_overridden and the materialiser keeps its header", Errors: []int{400, 404, 409, 500}})
	api.Post(g, "/trips/{trip_id}/{action}", trackingController.TransitionTrip, api.Doc{Summary: "Start, complete or cancel a trip", Description: "start: planned → in_progress · complete: in_progress → completed · cancel: planned or in_progress → cancelled, its pending tasks cancelled and the trip is_overridden", Errors: []int{400, 404, 409, 500}})
	api.Post(g, "/trip-stops/{stop_id}/{action}", trackingController.TransitionTripStop, api.Doc{Summary: "Arrive at or skip a stop", Description: "Of an in-progress trip. skip turns the stop's pending pickups into no_show and its pending dropoffs into cancelled", Errors: []int{400, 404, 409, 500}})
	api.Post(g, "/trip-stop-tasks/{task_id}/{action}", trackingController.TransitionTripTask, api.Doc{Summary: "Mark a pickup or dropoff done, or the rider a no-show", Description: "Of an in-progress trip, at a stop not skipped. Idempotent on client_op_id: the same id again answers 200 and changes nothing; an id already used on another task is 409", Errors: []int{400, 404, 409, 500}})
	api.Get(g, "/trip-stop-tasks/{task_id}/proof", trackingController.GetTripTaskProof, api.Doc{Summary: "Link to a drop-off's proof photo", Description: "A 60-second signed link to the photo the driver took at the drop-off (MOBILE-032). An organization user reaches its riders' tasks, a guardian their own riders'; no photo, or a task out of scope, is 404", Errors: []int{400, 404}})
}
