package models

import (
	"time"
)

// DTOs for API requests/responses

type CreateProductDto struct {
	SKU         string  `json:"sku" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	BasePrice   float64 `json:"base_price" binding:"required,gt=0"`
	Variants    any     `json:"variants"`
	// PosVisible and OrdersVisible say where it is sold (INV-019 D3); absent means yes.
	PosVisible    *bool `json:"pos_visible"`
	OrdersVisible *bool `json:"orders_visible"`
	// Attributes is {code: value or list of values} (INV-024 D1).
	Attributes map[string]any `json:"attributes"`
	// ActedBy is who is writing: set server-side, never read from the body (INV-020 D1).
	ActedBy *string `json:"-"`
}

// UpdateProductDto carries the editable product fields plus, optionally, the
// whole variant tree. Variants is a pointer so an omitted key (leave the tree
// alone) stays distinguishable from {"groups": []} (remove every group).
type UpdateProductDto struct {
	Name        string                    `json:"name" binding:"required"`
	Description *string                   `json:"description"`
	BasePrice   float64                   `json:"base_price" binding:"required,gt=0"`
	Variants    *UpdateProductVariantsDto `json:"variants"`
	// PosVisible and OrdersVisible left out keep what they were (INV-019 D3).
	PosVisible    *bool `json:"pos_visible"`
	OrdersVisible *bool `json:"orders_visible"`
	// Attributes left out keep what they were; axes are written with the tree (INV-024).
	Attributes map[string]any `json:"attributes"`
	ActedBy    *string        `json:"-"`
}

type UpdateProductVariantsDto struct {
	Groups []UpdateVariantGroupDto `json:"groups"`
}

// UpdateVariantGroupDto and UpdateVariantOptionDto are diffed by ID: a nil ID
// is an insert, an ID belonging to the product is an update, and anything in
// the DB missing from the payload is deleted.
// AffectsInventory marks the group as an inventory axis (INV-008): every one of
// its options multiplies the product's SKUs. Deliberately not a pointer — an
// omitted key means FALSE, which is what the product form that predates INV-008
// sends and what the SP reads.
type UpdateVariantGroupDto struct {
	ID               *string                  `json:"id"`
	AttributeID      *string                  `json:"attribute_id" binding:"omitempty,uuid"`
	GroupType        string                   `json:"group_type" binding:"required"`
	IsRequired       bool                     `json:"is_required"`
	MaxSelections    int                      `json:"max_selections"`
	AffectsInventory bool                     `json:"affects_inventory"`
	Options          []UpdateVariantOptionDto `json:"options"`
}

type UpdateVariantOptionDto struct {
	ID       *string `json:"id"`
	Name     string  `json:"name" binding:"required"`
	Modifier float64 `json:"modifier"`
}

// RecordMovementDto binds POST /inventory/movements. SkuID is a pointer because
// omitting it is meaningful: on a product with no axis it means the product SKU,
// and on one whose stock lives on combinations the SP refuses it outright
// (INV-011 D1) rather than filling the unassigned bucket.
//
// Quantity is a magnitude, never a signed amount: the movement type decides which
// way stock moves (INV-013 D1), so gt=0 here and the SP applies the sign. Direction
// is a pointer for the same reason SkuID is — omitting it is the normal case, and
// only ADJUSTMENT and TRANSFER are refused for it.
type RecordMovementDto struct {
	ProductID     string   `json:"product_id" binding:"required,uuid"`
	SkuID         *string  `json:"sku_id" binding:"omitempty,uuid"`
	MovementType  string   `json:"movement_type" binding:"required,oneof=PURCHASE SALE ADJUSTMENT TRANSFER RETURN WASTE PRODUCTION"`
	Quantity      float64  `json:"quantity" binding:"required,gt=0"`
	Direction     *string  `json:"direction" binding:"omitempty,oneof=IN OUT"`
	ReferenceType *string  `json:"reference_type"`
	ReferenceID   *string  `json:"reference_id"`
	UnitCost      *float64 `json:"unit_cost"`
	Notes         *string  `json:"notes"`
	// WarehouseID left out is the default warehouse (INV-025 D1).
	WarehouseID *string `json:"warehouse_id" binding:"omitempty,uuid"`
}

// ListMovementsQuery binds GET /inventory/movements. Every filter is optional; the
// three typed ones are validated here because an unparseable value would otherwise
// reach Postgres as a failed cast and surface as a 500 instead of a 400.
type ListMovementsQuery struct {
	ProductID    *string `form:"product_id" binding:"omitempty,uuid"`
	SkuID        *string `form:"sku_id" binding:"omitempty,uuid"`
	MovementType *string `form:"movement_type" binding:"omitempty"`
	DateFrom     *string `form:"date_from" binding:"omitempty,datetime=2006-01-02"`
	DateTo       *string `form:"date_to" binding:"omitempty,datetime=2006-01-02"`
	// Search reads product, SKU, document and person; Group is document by default without a product_id (INV-021 D2).
	Search        string  `form:"search"`
	ReferenceType *string `form:"reference_type" binding:"omitempty"`
	Group         string  `form:"group" binding:"omitempty,oneof=document none"`
	Page          int     `form:"page,default=1" binding:"min=1"`
	PageSize      int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

// ListProductsQuery binds GET /inventory/products (APP-004). category_id is validated here so a
// malformed one is a 400, never a silently unfiltered list.
type ListProductsQuery struct {
	CategoryID *string `form:"category_id" binding:"omitempty,uuid"`
	Search     string  `form:"search"`
	// Status is active by default; Filter is one of the chips (INV-019 D1, D2).
	Status   string `form:"status,default=active" binding:"omitempty,oneof=active discontinued all"`
	Filter   string `form:"filter" binding:"omitempty,oneof=with_variants low out_of_stock pos_hidden"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
	// Attrs is attr[code]=a,b as {code: [a, b]}, set by the controller from the raw query (INV-024).
	Attrs *string `form:"-"`
}

type ListProductsResponse struct {
	Products []Product `json:"products"`
	// Counts behind the chips, over the same search and category: every filter but the chip itself.
	Counts     ProductCounts `json:"counts"`
	TotalCount int64         `json:"total_count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
}

type ListMovementsResponse struct {
	Movements  []InventoryMovement `json:"movements"`
	TotalCount int64               `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}

type CreateProductResponse struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Message   string `json:"message"`
}

type UpdateProductResponse struct {
	ProductID string `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Message   string `json:"message"`
}

type RecordMovementResponse struct {
	MovementID       string  `json:"movement_id"`
	NewStockQuantity float64 `json:"new_stock_quantity"`
	Message          string  `json:"message"`
}

type UpdateReorderLevelDto struct {
	ReorderLevel float64 `json:"reorder_level" binding:"gte=0"`
	// SkuID names one combination; none is the product's default bucket.
	SkuID *string `json:"sku_id" binding:"omitempty,uuid"`
	// WarehouseID left out is the default warehouse (INV-025 D1).
	WarehouseID *string `json:"warehouse_id" binding:"omitempty,uuid"`
}

type UpdateReorderLevelResponse struct {
	ProductID    string  `json:"product_id"`
	ReorderLevel float64 `json:"reorder_level"`
	CurrentQty   float64 `json:"current_quantity"`
	Status       string  `json:"status"`
	Message      string  `json:"message"`
}

type StockReservationResponse struct {
	ReservedQuantity  float64 `json:"reserved_quantity"`
	AvailableQuantity float64 `json:"available_quantity"`
	Status            string  `json:"status"`
	Message           string  `json:"message"`
}

// SkuID follows the same rule as RecordMovementDto's: optional on a product with
// no axis, required by the SP on one stocked by combination (INV-011 D1).
type ReserveStockDto struct {
	ProductID string  `json:"product_id" binding:"required,uuid"`
	SkuID     *string `json:"sku_id" binding:"omitempty,uuid"`
	Quantity  float64 `json:"quantity" binding:"required,gt=0"`
}

type ReleaseReservedStockDto struct {
	ProductID string  `json:"product_id" binding:"required,uuid"`
	SkuID     *string `json:"sku_id" binding:"omitempty,uuid"`
	Quantity  float64 `json:"quantity" binding:"required,gt=0"`
}

// Domain Models

// ProductCategoryRef is the trimmed category shape a product row carries — enough
// to label and link it, not the whole category.
type ProductCategoryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Product struct {
	ID          string               `db:"id" json:"id"`
	SKU         string               `db:"sku" json:"sku"`
	Name        string               `db:"name" json:"name"`
	Description *string              `db:"description" json:"description"`
	BasePrice   float64              `db:"base_price" json:"base_price"`
	HasVariants bool                 `db:"has_variants" json:"has_variants"`
	Status      string               `db:"status" json:"status"`
	CreatedAt   time.Time            `db:"created_at" json:"created_at"`
	Categories  []ProductCategoryRef `json:"categories"`
	// What the list shows beside the product (INV-019 step 1).
	AvailableQuantity float64             `json:"available_quantity"`
	StockStatus       string              `json:"stock_status"`
	VariantsCount     int                 `json:"variants_count"`
	Axes              []ProductAxis       `json:"axes"`
	MinPrice          float64             `json:"min_price"`
	MaxPrice          float64             `json:"max_price"`
	PrimaryImageURL   *string             `json:"primary_image_url"`
	PosVisible        bool                `json:"pos_visible"`
	Units             []ProductUnit       `json:"units"`
	Attributes        map[string][]string `json:"attributes"`
}

// ProductAxis is one inventory axis of a product with the options it has.
type ProductAxis struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

// ProductCounts are the list's chips.
type ProductCounts struct {
	All          int64 `json:"all"`
	WithVariants int64 `json:"with_variants"`
	Low          int64 `json:"low"`
	OutOfStock   int64 `json:"out_of_stock"`
	PosHidden    int64 `json:"pos_hidden"`
}

// SetProductStatusDto discontinues or reactivates a product (INV-019 D1).
type SetProductStatusDto struct {
	Status string `json:"status" binding:"required,oneof=active discontinued"`
}

// SetProductStatusResponse is the product's id and new status.
type SetProductStatusResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// BulkProductChange is what one bulk action sets on every selected product.
type BulkProductChange struct {
	Status         *string  `json:"status" binding:"omitempty,oneof=active discontinued"`
	BasePrice      *float64 `json:"base_price" binding:"omitempty,gt=0"`
	PriceChangePct *float64 `json:"price_change_pct" binding:"omitempty,gt=-100"`
	CategoryID     *string  `json:"category_id" binding:"omitempty,uuid"`
	PosVisible     *bool    `json:"pos_visible"`
}

// BulkUpdateProductsDto is one change to at most 100 products.
type BulkUpdateProductsDto struct {
	IDs []string          `json:"ids" binding:"required,min=1,max=100,dive,uuid"`
	Set BulkProductChange `json:"set" binding:"required"`
}

// BulkUpdateProductsResponse says how many changed.
type BulkUpdateProductsResponse struct {
	Updated int `json:"updated"`
}

type ProductMedia struct {
	ID        string  `json:"id"`
	MediaType string  `json:"media_type"`
	URL       string  `json:"url"`
	AltText   *string `json:"alt_text"`
	IsPrimary bool    `json:"is_primary"`
	SortOrder int     `json:"sort_order"`
}

type VariantOption struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	PriceModifier float64        `json:"price_modifier"`
	IsAvailable   bool           `json:"is_available"`
	SortOrder     int            `json:"sort_order"`
	Media         []ProductMedia `json:"media"`
}

type VariantGroup struct {
	ID               string          `json:"id"`
	AttributeID      *string         `json:"attribute_id"`
	GroupType        string          `json:"group_type"`
	IsRequired       bool            `json:"is_required"`
	MaxSelections    int             `json:"max_selections"`
	SortOrder        int             `json:"sort_order"`
	AffectsInventory bool            `json:"affects_inventory"`
	Options          []VariantOption `json:"options"`
}

type ProductDetail struct {
	ID          string         `json:"id"`
	SKU         string         `json:"sku"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	BasePrice   float64        `json:"base_price"`
	HasVariants bool           `json:"has_variants"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	Media       []ProductMedia `json:"media"`
	Variants    []VariantGroup `json:"variants"`
	// UpdatedAt and UpdatedByName say who last changed it (INV-020 D1); no name on a row nobody touched since.
	UpdatedAt     *time.Time          `json:"updated_at"`
	UpdatedByName *string             `json:"updated_by_name"`
	PosVisible    bool                `json:"pos_visible"`
	OrdersVisible bool                `json:"orders_visible"`
	Units         []ProductUnit       `json:"units"`
	Attributes    map[string][]string `json:"attributes"`
}

type AddProductMediaDto struct {
	VariantOptionID *string `json:"variant_option_id"`
	MediaType       string  `json:"media_type" binding:"required,oneof=image video"`
	URL             string  `json:"url" binding:"required,url"`
	AltText         *string `json:"alt_text"`
	IsPrimary       bool    `json:"is_primary"`
	SortOrder       int     `json:"sort_order"`
}

type AddProductMediaResponse struct {
	MediaID string `json:"media_id"`
	Message string `json:"message"`
}

type ProductStock struct {
	CurrentQuantity   float64   `db:"current_quantity" json:"current_quantity"`
	ReservedQuantity  float64   `db:"reserved_quantity" json:"reserved_quantity"`
	AvailableQuantity float64   `db:"available_quantity" json:"available_quantity"`
	ReorderLevel      float64   `db:"reorder_level" json:"reorder_level"`
	Status            string    `db:"status" json:"status"` // ok, low, critical, out_of_stock
	LastUpdatedAt     time.Time `db:"last_updated_at" json:"last_updated_at"`
}

// ---------------------------------------------------------------------------
// SKUs — the sellable combinations (INV-008)
// ---------------------------------------------------------------------------

// ProductSkuOption is one axis of a combination: which option of which group.
type ProductSkuOption struct {
	GroupID    string `json:"group_id"`
	GroupType  string `json:"group_type"`
	OptionID   string `json:"option_id"`
	OptionName string `json:"option_name"`
}

// ProductSku is one row of the per-SKU stock table. Sellable is INV-008 D6:
// the SKU can only narrow, never widen, what the product's status allows.
type ProductSku struct {
	SkuID             string             `json:"sku_id"`
	SKU               string             `json:"sku"`
	CombinationKey    string             `json:"combination_key"`
	IsDefault         bool               `json:"is_default"`
	Status            string             `json:"status"`
	Sellable          bool               `json:"sellable"`
	PriceModifier     float64            `json:"price_modifier"`
	Options           []ProductSkuOption `json:"options"`
	CurrentQuantity   float64            `json:"current_quantity"`
	ReservedQuantity  float64            `json:"reserved_quantity"`
	AvailableQuantity float64            `json:"available_quantity"`
	ReorderLevel      float64            `json:"reorder_level"`
	StockStatus       string             `json:"stock_status"`
}

// ListProductSkusQuery binds GET /inventory/products/:id/skus. The default page
// size is the combination cap, so a plain call returns the whole set.
type ListProductSkusQuery struct {
	Page     int `form:"page,default=1" binding:"min=1"`
	PageSize int `form:"page_size,default=100" binding:"min=1,max=100"`
}

type ListProductSkusResponse struct {
	Skus       []ProductSku `json:"skus"`
	TotalCount int64        `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

// GenerateSkusResponse reports what the generator found and what it added. A
// second run over the same tree returns CreatedCount 0 and is not an error.
type GenerateSkusResponse struct {
	ProductID        string `json:"product_id"`
	AxisCount        int    `json:"axis_count"`
	CombinationCount int    `json:"combination_count"`
	CreatedCount     int    `json:"created_count"`
	ExistingCount    int    `json:"existing_count"`
	StockByVariant   bool   `json:"stock_by_variant"`
	Message          string `json:"message"`
}

// CreateCombinationDto names one combination of a product the way a catalogue
// row does (INV-016 D6): the axes in the file's column order, each with the
// option that row holds and the price modifier that option carries (D3). SKU is
// the code verbatim when the row filled `variant_sku` and nil when it did not,
// and the SP then derives the one sp_generate_product_skus would have.
type CreateCombinationDto struct {
	Axes []CombinationAxisDto `json:"axes"`
	SKU  *string              `json:"sku,omitempty"`
}

// CombinationAxisDto is one axis column of one row.
type CombinationAxisDto struct {
	Axis     string  `json:"axis"`
	Option   string  `json:"option"`
	Modifier float64 `json:"modifier"`
}

// CreateCombinationResponse is the combination that was created: the SKU row an
// opening count, a lot or an image then hangs off.
type CreateCombinationResponse struct {
	ProductID string `json:"product_id"`
	SkuID     string `json:"sku_id"`
	SKU       string `json:"sku"`
	AxisCount int    `json:"axis_count"`
}

// RedistributeStockDto moves the whole default-SKU bucket onto combinations. The
// amounts must sum to exactly what the bucket holds (INV-008 D4) — the SP checks
// that, since only it can read the quantity under a lock.
type RedistributeStockDto struct {
	Targets []RedistributeTargetDto `json:"targets" binding:"required,min=1,dive"`
}

type RedistributeTargetDto struct {
	SkuID    string  `json:"sku_id" binding:"required,uuid"`
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
}

type RedistributeStockResponse struct {
	ProductID     string  `json:"product_id"`
	DefaultSkuID  string  `json:"default_sku_id"`
	MovedQuantity float64 `json:"moved_quantity"`
	TargetCount   int     `json:"target_count"`
	Message       string  `json:"message"`
}

type InventoryMovement struct {
	ID            string    `db:"id" json:"id"`
	ProductID     string    `db:"product_id" json:"product_id"`
	MovementType  string    `db:"movement_type" json:"movement_type"`
	Quantity      float64   `db:"quantity" json:"quantity"`
	ReferenceType *string   `db:"reference_type" json:"reference_type"`
	ReferenceID   *string   `db:"reference_id" json:"reference_id"`
	UnitCost      *float64  `db:"unit_cost" json:"unit_cost"`
	Notes         *string   `db:"notes" json:"notes"`
	CreatedBy     *string   `db:"created_by" json:"created_by"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	// Which combination moved. Both are nullable: rows written before INV-007
	// carry no SKU and the trail still has to show them (INV-011 D2).
	SkuID *string `db:"sku_id" json:"sku_id"`
	SKU   *string `db:"sku" json:"sku"`
	// What the ledger reads with no other query (INV-021). MoreCount is how many more lines the row folds.
	ProductName    string  `json:"product_name"`
	SkuLabel       *string `json:"sku_label"`
	CreatedByName  *string `json:"created_by_name"`
	ReferenceLabel *string `json:"reference_label"`
	MoreCount      int     `json:"more_count"`
}
