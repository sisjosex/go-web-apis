package models

import "time"

// CountPlace names the warehouse or category of a count.
type CountPlace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CountRow is one row of GET /inventory/counts.
type CountRow struct {
	ID               string      `json:"id"`
	Number           string      `json:"number"`
	Status           string      `json:"status"`
	Blind            bool        `json:"blind"`
	AdjustmentNumber *string     `json:"adjustment_number"`
	StartedAt        time.Time   `json:"started_at"`
	Warehouse        CountPlace  `json:"warehouse"`
	Category         *CountPlace `json:"category"`
	CreatedByName    *string     `json:"created_by_name"`
	TotalLines       int64       `json:"total_lines"`
	CountedLines     int64       `json:"counted_lines"`
	WithDifference   int64       `json:"with_difference"`
}

// CountChips are the numbers behind the list's chips.
type CountChips struct {
	Open   int64 `json:"open"`
	Review int64 `json:"review"`
	Closed int64 `json:"closed"`
}

// ListCountsQuery is its filters; status takes several, comma-separated.
type ListCountsQuery struct {
	Status   string `form:"status"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ListCountsResponse is GET /inventory/counts.
type ListCountsResponse struct {
	Counts     []CountRow `json:"counts"`
	Chips      CountChips `json:"chips"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// CountLine is a line of a count; what was expected is left out for a blind counter (D4).
type CountLine struct {
	SkuID       string   `json:"sku_id"`
	ProductName string   `json:"product_name"`
	SKU         string   `json:"sku"`
	Label       *string  `json:"label"`
	Expected    *float64 `json:"expected"`
	Counted     *float64 `json:"counted"`
	Difference  *float64 `json:"difference"`
	UnitCost    *float64 `json:"unit_cost"`
}

// CountProgress is how far a count went.
type CountProgress struct {
	Counted         int64    `json:"counted"`
	Total           int64    `json:"total"`
	WithDifference  *int64   `json:"with_difference"`
	DifferenceValue *float64 `json:"difference_value"`
}

// CountDetail is GET /inventory/counts/:id.
type CountDetail struct {
	ID               string        `json:"id"`
	Number           string        `json:"number"`
	Status           string        `json:"status"`
	Blind            bool          `json:"blind"`
	AdjustmentNumber *string       `json:"adjustment_number"`
	StartedAt        time.Time     `json:"started_at"`
	Warehouse        CountPlace    `json:"warehouse"`
	Category         *CountPlace   `json:"category"`
	CreatedByName    *string       `json:"created_by_name"`
	Lines            []CountLine   `json:"lines"`
	Progress         CountProgress `json:"progress"`
}

// CreateCountDto is POST /inventory/counts.
type CreateCountDto struct {
	WarehouseID *string `json:"warehouse_id" binding:"omitempty,uuid"`
	CategoryID  *string `json:"category_id" binding:"omitempty,uuid"`
	Blind       bool    `json:"blind"`
}

// CountLineDto is what was counted on one line.
type CountLineDto struct {
	SkuID   string   `json:"sku_id" binding:"required,uuid"`
	Counted *float64 `json:"counted" binding:"required,gte=0"`
}

// SetCountLinesDto is PATCH /inventory/counts/:id/lines.
type SetCountLinesDto struct {
	Lines []CountLineDto `json:"lines" binding:"required,min=1,max=200,dive"`
}

// ScanCountDto is POST /inventory/counts/:id/scan.
type ScanCountDto struct {
	Code string `json:"code" binding:"required,max=64"`
}

// CloseCountResponse is POST /inventory/counts/:id/close.
type CloseCountResponse struct {
	Status           string `json:"status"`
	AdjustmentNumber string `json:"adjustment_number"`
}
