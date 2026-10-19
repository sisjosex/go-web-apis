package models

import "github.com/google/uuid"

// UsageDay is one tenant's usage on one day (TRACK-013 D3).
type UsageDay struct {
	Day            string `json:"day"`
	VehiclesActive int    `json:"vehicles_active"`
	TripsCompleted int    `json:"trips_completed"`
	Positions      int64  `json:"positions"`
	RidersBoarded  int    `json:"riders_boarded"`
}

// TenantUsage is one tenant's days in a range with the range's peak and mean active vehicles.
type TenantUsage struct {
	TenantID          uuid.UUID  `json:"tenant_id"`
	Slug              string     `json:"slug"`
	Name              string     `json:"name"`
	Days              []UsageDay `json:"days"`
	VehiclesActiveMax int        `json:"vehicles_active_max"`
	VehiclesActiveAvg float64    `json:"vehicles_active_avg"`
	TripsCompleted    int        `json:"trips_completed"`
}

// PlatformUsageQuery is GET /platform/usage's query: dates YYYY-MM-DD, the last 30 days by default.
type PlatformUsageQuery struct {
	From     *string `form:"from" binding:"omitempty,datetime=2006-01-02"`
	To       *string `form:"to" binding:"omitempty,datetime=2006-01-02"`
	TenantID *string `form:"tenant_id" binding:"omitempty,uuid"`
}

// PlatformUsageResponse is GET /platform/usage's body.
type PlatformUsageResponse struct {
	Tenants []TenantUsage `json:"tenants"`
}
