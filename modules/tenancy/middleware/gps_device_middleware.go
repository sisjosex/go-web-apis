package middleware

import (
	"context"
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/repositories"

	"github.com/gin-gonic/gin"
)

// DeviceTokenHeader carries a GPS device's token (TRACK-010).
const DeviceTokenHeader = "X-Device-Token"

// GPSVehicleKey is where GPSDeviceMiddleware leaves the device's vehicle id (a uuid.UUID).
const GPSVehicleKey = "gps_vehicle_id"

// GPSDeviceMiddleware authenticates a GPS device by its token instead of a user and a slug: the token
// names its tenant, and its sha256 in that tenant names the vehicle — one platform query. It sets the same context a
// tenant chain sets — tenant_id, tenant_access, the tenant database — plus the vehicle, with
// tenant_user_role RoleGPSDevice. A device request never runs any other route.
func GPSDeviceMiddleware(devices interfaces.GPSDeviceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, hash, ok := repositories.ParseGPSDeviceToken(c.GetHeader(DeviceTokenHeader))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.GPSDeviceUnauthorized))
			return
		}
		access, vehicleID, err := devices.Resolve(c.Request.Context(), tenantID, hash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
			return
		}
		if access == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, tenancyErrors.GPSDeviceUnauthorized))
			return
		}
		if !access.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantInactive))
			return
		}
		if access.IsSuspended {
			c.AbortWithStatusJSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.TenantSuspended))
			return
		}

		c.Set("tenant_id", access.TenantID.String())
		c.Set("tenant_slug", access.Slug)
		c.Set("tenant_database_url", access.DatabaseURL)
		c.Set("tenant_schema_name", access.SchemaName)
		c.Set("tenant_user_role", access.UserRole)
		c.Set("tenant_access", access)
		c.Set(GPSVehicleKey, *vehicleID)
		if access.DatabaseURL != nil && *access.DatabaseURL != "" {
			ctx := context.WithValue(c.Request.Context(), coreServices.TenantDatabaseURLKey, *access.DatabaseURL)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
