package routes

import (
	"github.com/gin-gonic/gin"

	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
	"josex/web/modules/import/controllers"
)

// RegisterImportRoutes registers the generic import endpoints, scoped to a tenant.
// Callers pass the tenant middleware (TenantMiddlewareFromHeader when tenancy is
// enabled, or a no-op handler otherwise).
func RegisterImportRoutes(
	router *gin.RouterGroup,
	importController *controllers.ImportController,
	jwtService services.JWTService,
	tenantMiddleware gin.HandlerFunc,
) {
	importRoutes := router.Group("/import")
	importRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	importRoutes.Use(tenantMiddleware)
	{
		api.Get(importRoutes, "/meta", importController.Meta, api.Doc{Summary: "Import limits", Description: "Returns the configured upload constraints (max file size, max rows) so the UI hint stays in sync with the server.", Errors: []int{401}})
		api.Get(importRoutes, "/resources", importController.Resources, api.Doc{Summary: "Resources a scope reaches", Description: "Lists the import resources a scope of resource or module keys reaches, with the module that owns each, in detection order. An empty scope lists every registered resource.", Errors: []int{401}})
		api.Get(importRoutes, "/template", importController.Template, api.Doc{Summary: "Download a CSV template", Description: "Returns a CSV containing just the header row for a resource, built from its descriptor's columns.", Errors: []int{400, 401}})
		api.Get(importRoutes, "/schema", importController.Schema, api.Doc{Summary: "Column schema for a resource", Description: "Returns the resource's columns with their type, required flag and optional validation pattern, so a client can render and check an editor per column without knowing the resource.", Errors: []int{400, 401}})
		api.Get(importRoutes, "/mapping", importController.Mapping, api.Doc{Summary: "Remembered column mapping", Description: "The {folded heading: field} remembered for a file with these headings in this business, or null (IMPORT-002 D3).", Errors: []int{400, 401}})
		api.Get(importRoutes, "/runs", importController.Runs, api.Doc{Summary: "Import history", Description: "The business's confirmed imports, newest first, with their counts and who ran each; one resource when set.", Errors: []int{401}})
		api.Post(importRoutes, "/detect", importController.Detect, api.Doc{Summary: "Detect a CSV's resource", Description: "Resolves the resource a CSV belongs to from its header line alone, inside the optional scope, without reading or validating any row — so a client can queue several files in import order before running them.", Errors: []int{400, 401}})
		api.Post(importRoutes, "/validate", importController.Validate, api.Doc{Summary: "Dry-run validate an uploaded CSV", Description: "Detects the resource from the CSV header (or the explicit resource field), validates every row, and returns per-row valid|invalid|duplicate feedback keyed by CSV line number. Writes nothing.", Errors: []int{400, 401}})
		api.Post(importRoutes, "", importController.Import, api.Doc{Summary: "Process an uploaded CSV", Description: "Detects the resource, creates each valid row, and returns per-row created|failed|skipped outcomes plus a persisted run summary (who/when/counts/warnings).", Errors: []int{400, 401}})
	}
}
