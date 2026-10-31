package models

import "time"

// WarehouseManager is who answers for a warehouse.
type WarehouseManager struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Warehouse is where stock is kept, with what it holds (INV-025).
type Warehouse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Address   *string           `json:"address"`
	City      *string           `json:"city"`
	Kind      string            `json:"kind"`
	Manager   *WarehouseManager `json:"manager"`
	IsDefault bool              `json:"is_default"`
	Status    string            `json:"status"`
	Value     float64           `json:"value"`
	Units     float64           `json:"units"`
	BelowMin  int64             `json:"below_min"`
}

// ListWarehousesResponse is GET /inventory/warehouses.
type ListWarehousesResponse struct {
	Warehouses []Warehouse `json:"warehouses"`
}

// UpsertWarehouseDto is POST and PUT /inventory/warehouses.
type UpsertWarehouseDto struct {
	Name          string  `json:"name" binding:"required,max=120"`
	Address       *string `json:"address" binding:"omitempty,max=200"`
	City          *string `json:"city" binding:"omitempty,max=80"`
	Kind          string  `json:"kind" binding:"omitempty,oneof=store storage vehicle"`
	ManagerUserID *string `json:"manager_user_id" binding:"omitempty,uuid"`
}

// SetWarehouseStatusDto archives or reactivates a warehouse.
type SetWarehouseStatusDto struct {
	Status string `json:"status" binding:"required,oneof=active archived"`
}

// WarehouseQuantity is how much of something one warehouse holds.
type WarehouseQuantity struct {
	WarehouseID string  `json:"warehouse_id"`
	Quantity    float64 `json:"quantity"`
}

// NextLot is the lot of a product that expires first.
type NextLot struct {
	LotNumber  string `json:"lot_number"`
	ExpiryDate string `json:"expiry_date"`
}

// StockItem is one row of the stock screen.
type StockItem struct {
	ProductID        string              `json:"product_id"`
	Name             string              `json:"name"`
	SKU              string              `json:"sku"`
	VariantsCount    int                 `json:"variants_count"`
	LowVariants      int                 `json:"low_variants"`
	ByWarehouse      []WarehouseQuantity `json:"by_warehouse"`
	Total            float64             `json:"total"`
	ReorderLevel     float64             `json:"reorder_level"`
	Status           string              `json:"status"`
	ShortWarehouseID *string             `json:"short_warehouse_id"`
	NextLot          *NextLot            `json:"next_lot"`
}

// StockCounts are the numbers behind the screen's chips.
type StockCounts struct {
	All        int64 `json:"all"`
	Low        int64 `json:"low"`
	OutOfStock int64 `json:"out_of_stock"`
	Expiring   int64 `json:"expiring"`
	Stale      int64 `json:"stale"`
}

// ListStockQuery is GET /inventory/stock's filters.
type ListStockQuery struct {
	WarehouseID *string `form:"warehouse_id" binding:"omitempty,uuid"`
	Search      string  `form:"search"`
	CategoryID  *string `form:"category_id" binding:"omitempty,uuid"`
	Filter      string  `form:"filter" binding:"omitempty,oneof=low out_of_stock expiring stale"`
	Page        int     `form:"page,default=1" binding:"min=1"`
	PageSize    int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ListStockResponse is GET /inventory/stock.
type ListStockResponse struct {
	Items      []StockItem `json:"items"`
	Counts     StockCounts `json:"counts"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

// StockSku is a combination in the row a product expands into.
type StockSku struct {
	SkuID        string              `json:"sku_id"`
	Label        string              `json:"label"`
	ByWarehouse  []WarehouseQuantity `json:"by_warehouse"`
	Total        float64             `json:"total"`
	ReorderLevel float64             `json:"reorder_level"`
}

// ListStockSkusResponse is GET /inventory/stock/:product_id/skus.
type ListStockSkusResponse struct {
	Skus []StockSku `json:"skus"`
}

// TransferFrom is a warehouse that can spare what another lacks.
type TransferFrom struct {
	WarehouseID string  `json:"warehouse_id"`
	Quantity    float64 `json:"quantity"`
}

// ReorderItem is a row of "Reponer pronto".
type ReorderItem struct {
	ProductID     string        `json:"product_id"`
	SkuID         string        `json:"sku_id"`
	Name          string        `json:"name"`
	SKU           string        `json:"sku"`
	WarehouseID   string        `json:"warehouse_id"`
	WarehouseName string        `json:"warehouse_name"`
	Quantity      float64       `json:"quantity"`
	ReorderLevel  float64       `json:"reorder_level"`
	Suggested     float64       `json:"suggested"`
	TransferFrom  *TransferFrom `json:"transfer_from"`
}

// ReorderList is the most urgent rows and how many there are.
type ReorderList struct {
	Items []ReorderItem `json:"items"`
	Total int64         `json:"total"`
}

// TopSeller is one of the week's best sellers.
type TopSeller struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
}

// InventorySummary is GET /inventory/summary, the inventory home.
type InventorySummary struct {
	WarehousesCount int64       `json:"warehouses_count"`
	Value           float64     `json:"value"`
	Products        int64       `json:"products"`
	Variants        int64       `json:"variants"`
	Low             int64       `json:"low"`
	OutOfStock      int64       `json:"out_of_stock"`
	Expiring        int64       `json:"expiring"`
	Reorder         ReorderList `json:"reorder"`
	TopSellers      []TopSeller `json:"top_sellers"`
}

// InventoryNavBadges are the numbers beside the inventory menu.
type InventoryNavBadges struct {
	StockLow   int64 `json:"stock_low"`
	CountsOpen int64 `json:"counts_open"`
}

// TransferLineDto is a line of a new transfer.
type TransferLineDto struct {
	SkuID    string  `json:"sku_id" binding:"required,uuid"`
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
}

// CreateTransferDto is POST /inventory/transfers.
type CreateTransferDto struct {
	FromWarehouseID string            `json:"from_warehouse_id" binding:"required,uuid"`
	ToWarehouseID   string            `json:"to_warehouse_id" binding:"required,uuid"`
	Lines           []TransferLineDto `json:"lines" binding:"required,min=1,dive"`
	Notes           *string           `json:"notes"`
}

// ShipTransferDto is POST /inventory/transfers/:id/ship.
type ShipTransferDto struct {
	CarrierNote *string `json:"carrier_note"`
}

// ReceivedLineDto is what was confirmed of one line.
type ReceivedLineDto struct {
	SkuID            string  `json:"sku_id" binding:"required,uuid"`
	ReceivedQuantity float64 `json:"received_quantity" binding:"gte=0"`
}

// ReceiveTransferDto is POST /inventory/transfers/:id/receive; a line left out arrived whole.
type ReceiveTransferDto struct {
	Lines []ReceivedLineDto `json:"lines" binding:"dive"`
}

// TransferPlace is one end of a transfer.
type TransferPlace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TransferLine is a line of a transfer.
type TransferLine struct {
	SkuID            string   `json:"sku_id"`
	ProductID        string   `json:"product_id"`
	Label            string   `json:"label"`
	Quantity         float64  `json:"quantity"`
	ReceivedQuantity *float64 `json:"received_quantity"`
	// Lots are those the units left in, first to expire (INV-027).
	Lots []TransferLot `json:"lots"`
}

// TransferLot is what a line took from one lot.
type TransferLot struct {
	LotNumber  string  `json:"lot_number"`
	ExpiryDate string  `json:"expiry_date"`
	Quantity   float64 `json:"quantity"`
}

// TransferStep is one moment of its timeline.
type TransferStep struct {
	Step   string     `json:"step"`
	At     *time.Time `json:"at"`
	ByName *string    `json:"by_name"`
	Note   *string    `json:"note"`
}

// Transfer is GET /inventory/transfers/:id.
type Transfer struct {
	ID       string         `json:"id"`
	Number   string         `json:"number"`
	Status   string         `json:"status"`
	Notes    *string        `json:"notes"`
	From     TransferPlace  `json:"from"`
	To       TransferPlace  `json:"to"`
	Lines    []TransferLine `json:"lines"`
	Units    float64        `json:"units"`
	Products int64          `json:"products"`
	Timeline []TransferStep `json:"timeline"`
}

// TransferRow is one row of GET /inventory/transfers.
type TransferRow struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	FromName   string    `json:"from_name"`
	ToName     string    `json:"to_name"`
	Units      float64   `json:"units"`
	PreparedAt time.Time `json:"prepared_at"`
}

// ListTransfersQuery is its filters.
type ListTransfersQuery struct {
	Status   string `form:"status" binding:"omitempty,oneof=prepared in_transit received cancelled"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ListTransfersResponse is GET /inventory/transfers.
type ListTransfersResponse struct {
	Transfers  []TransferRow `json:"transfers"`
	TotalCount int64         `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
}
