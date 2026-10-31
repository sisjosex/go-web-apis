package utils

import (
	"context"
	"sync"
	"time"

	"josex/web/modules/core/registry"
	tenancyModels "josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
)

const defaultZone = "America/La_Paz"

// zones keeps each location once: LoadLocation reads a file on every call.
var zones sync.Map

// TenantToday is the request's tenant's calendar date (INV-018 D1), from its general.timezone in the
// shared settings cache: no query while warm.
func TenantToday(c *gin.Context) time.Time {
	return TodayOf(c.Request.Context(), c.GetString("tenant_slug"))
}

// TodayOf is TenantToday outside a request handler, for a caller that holds the tenant's slug.
func TodayOf(ctx context.Context, slug string) time.Time {
	zone, _ := registry.TenantSetting(ctx, slug, tenancyModels.SettingTimezone).(string)
	return DateIn(time.Now(), zone)
}

// DateIn is the calendar date in zone at the instant now, as midnight UTC so a driver encoding it as a
// date cannot move it. An unknown zone reads as the default one.
func DateIn(now time.Time, zone string) time.Time {
	year, month, day := now.In(location(zone)).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func location(zone string) *time.Location {
	if cached, ok := zones.Load(zone); ok {
		return cached.(*time.Location)
	}
	loc, err := time.LoadLocation(zone)
	if err != nil || zone == "" {
		if zone == defaultZone {
			return time.UTC
		}
		return location(defaultZone)
	}
	zones.Store(zone, loc)
	return loc
}
