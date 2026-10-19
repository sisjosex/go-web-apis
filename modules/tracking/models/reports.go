package models

import "github.com/google/uuid"

// The reports of TRACK-013: reads over the aggregates the trip close and the nightly pass roll.

// Report names, the :report segment of the export.
const (
	ReportAttendance  = "attendance"
	ReportPunctuality = "punctuality"
	ReportFleetUsage  = "fleet-usage"
	ReportIncidents   = "incidents"
)

// DayUsage is what a tenant used on one day (TRACK-013 D3): the row the nightly pass writes to the
// platform's tenancy.usage_daily.
type DayUsage struct {
	VehiclesActive int
	TripsCompleted int
	Positions      int64
	RidersBoarded  int
}

// ReportQuery binds every report's query; each report reads the filters it has. Dates are YYYY-MM-DD,
// the last 30 days by default.
type ReportQuery struct {
	From           *string `form:"from" binding:"omitempty,datetime=2006-01-02"`
	To             *string `form:"to" binding:"omitempty,datetime=2006-01-02"`
	RouteID        *string `form:"route_id" binding:"omitempty,uuid"`
	OrganizationID *string `form:"organization_id" binding:"omitempty,uuid"`
	RiderID        *string `form:"rider_id" binding:"omitempty,uuid"`
	GroupBy        string  `form:"group_by,default=vehicle" binding:"oneof=vehicle driver"`
	AlertType      *string `form:"alert_type" binding:"omitempty,oneof=delay breakdown cancellation emergency other"`
	Page           int     `form:"page,default=1" binding:"min=1"`
	PageSize       int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ReportPage is one report SP's answer before it is decoded: the page's rows, the range's summary and
// how many rows the filters match.
type ReportPage struct {
	Rows       []byte
	Summary    []byte
	TotalCount int64
}

// AttendanceRow is one rider over the range.
type AttendanceRow struct {
	RiderID          uuid.UUID `json:"rider_id"`
	RiderName        string    `json:"rider_name"`
	OrganizationName *string   `json:"organization_name"`
	Days             int       `json:"days"`
	Boarded          int       `json:"boarded"`
	NoShows          int       `json:"no_shows"`
	Absent           int       `json:"absent"`
	Cancelled        int       `json:"cancelled"`
	AttendancePct    *float64  `json:"attendance_pct"`
}

type AttendanceSummary struct {
	Riders        int      `json:"riders"`
	Days          int      `json:"days"`
	Boarded       int      `json:"boarded"`
	NoShows       int      `json:"no_shows"`
	Absent        int      `json:"absent"`
	AttendancePct *float64 `json:"attendance_pct"`
}

type AttendanceReport struct {
	Rows       []AttendanceRow   `json:"rows"`
	Summary    AttendanceSummary `json:"summary"`
	TotalCount int64             `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}

// WorstStop is a route's stop reached latest, by p90 over the last weeks.
type WorstStop struct {
	StopName    string  `json:"stop_name"`
	P90DelayMin float64 `json:"p90_delay_min"`
}

// PunctualityRow is one route over the range.
type PunctualityRow struct {
	RouteID          uuid.UUID  `json:"route_id"`
	RouteName        string     `json:"route_name"`
	Direction        string     `json:"direction"`
	OrganizationName *string    `json:"organization_name"`
	Trips            int        `json:"trips"`
	OnTime           int        `json:"on_time"`
	OnTimePct        float64    `json:"on_time_pct"`
	AvgStartDelayMin *float64   `json:"avg_start_delay_min"`
	MaxStartDelayMin *int       `json:"max_start_delay_min"`
	AvgStopDelayMin  *float64   `json:"avg_stop_delay_min"`
	WorstStop        *WorstStop `json:"worst_stop"`
}

type PunctualitySummary struct {
	Trips            int      `json:"trips"`
	OnTimePct        *float64 `json:"on_time_pct"`
	AvgStartDelayMin *float64 `json:"avg_start_delay_min"`
	MaxStartDelayMin *int     `json:"max_start_delay_min"`
}

type PunctualityReport struct {
	Rows       []PunctualityRow   `json:"rows"`
	Summary    PunctualitySummary `json:"summary"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}

// FleetUsageRow is one vehicle (label = plate) or driver (label = name) over the range.
type FleetUsageRow struct {
	ID              uuid.UUID `json:"id"`
	Label           string    `json:"label"`
	CompanyName     *string   `json:"company_name"`
	Trips           int       `json:"trips"`
	Km              float64   `json:"km"`
	Hours           float64   `json:"hours"`
	RidersBoarded   int       `json:"riders_boarded"`
	DaysActive      int       `json:"days_active"`
	AvgOccupancyPct *float64  `json:"avg_occupancy_pct"`
}

// FleetUsageSummary carries vehicles or drivers, as the report was grouped.
type FleetUsageSummary struct {
	Vehicles      *int    `json:"vehicles,omitempty"`
	Drivers       *int    `json:"drivers,omitempty"`
	Trips         int     `json:"trips"`
	Km            float64 `json:"km"`
	Hours         float64 `json:"hours"`
	RidersBoarded int     `json:"riders_boarded"`
}

type FleetUsageReport struct {
	Rows       []FleetUsageRow   `json:"rows"`
	Summary    FleetUsageSummary `json:"summary"`
	TotalCount int64             `json:"total_count"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
}

type IncidentsSummary struct {
	Total         int            `json:"total"`
	Open          int            `json:"open"`
	ByType        map[string]int `json:"by_type"`
	AvgResolveMin *float64       `json:"avg_resolve_min"`
}

// IncidentsReport's rows are GET /tracking/alerts' rows.
type IncidentsReport struct {
	Rows       []RouteAlert     `json:"rows"`
	Summary    IncidentsSummary `json:"summary"`
	TotalCount int64            `json:"total_count"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
}
