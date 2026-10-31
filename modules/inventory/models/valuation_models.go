package models

// ValuationItem is one product's stock at cost; UnitCost and Value are null when nothing says what it cost.
type ValuationItem struct {
	ProductID string   `json:"product_id"`
	SKU       string   `json:"sku"`
	Name      string   `json:"name"`
	Quantity  float64  `json:"quantity"`
	UnitCost  *float64 `json:"unit_cost"`
	Value     *float64 `json:"value"`
}

// ValuationResponse is GET /inventory/reports/valuation.
type ValuationResponse struct {
	Items         []ValuationItem `json:"items"`
	TotalValue    float64         `json:"total_value"`
	UncostedCount int64           `json:"uncosted_count"`
	TotalCount    int64           `json:"total_count"`
	Page          int             `json:"page"`
	PageSize      int             `json:"page_size"`
}

// ValuationQuery is its filters.
type ValuationQuery struct {
	CategoryID *string `form:"category_id" binding:"omitempty,uuid"`
	Page       int     `form:"page,default=1" binding:"min=1"`
	PageSize   int     `form:"page_size,default=20" binding:"min=1,max=100"`
}
