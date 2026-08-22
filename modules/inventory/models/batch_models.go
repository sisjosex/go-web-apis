package models

import (
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// CreateBatchDto - DTO for creating a new batch
type CreateBatchDto struct {
	ProductID string `json:"product_id" binding:"required,uuidv4"`
	// SkuID - the combination the lot belongs to. A pointer because omitting it
	// and sending null must mean the same thing: let the SP resolve the default
	// (and refuse when the product is stocked by variant, INV-014 D1).
	SkuID           *string             `json:"sku_id" binding:"omitempty,uuid"`
	LotNumber       string              `json:"lot_number" binding:"required,min=1,max=100"`
	PurchaseDate    coreModels.DateOnly `json:"purchase_date" binding:"required"`
	ExpiryDate      coreModels.DateOnly `json:"expiry_date" binding:"required"`
	UnitCost        float64             `json:"unit_cost" binding:"required,gt=0"`
	InitialQuantity float64             `json:"initial_quantity" binding:"required,gt=0"`
}

// BatchResponse - Response model for batch
type BatchResponse struct {
	ID              uuid.UUID           `json:"id"`
	ProductID       uuid.UUID           `json:"product_id"`
	SkuID           *uuid.UUID          `json:"sku_id,omitempty"`
	Sku             *string             `json:"sku,omitempty"`
	LotNumber       string              `json:"lot_number"`
	PurchaseDate    coreModels.DateOnly `json:"purchase_date"`
	ExpiryDate      coreModels.DateOnly `json:"expiry_date"`
	UnitCost        float64             `json:"unit_cost"`
	InitialQuantity float64             `json:"initial_quantity"`
	CurrentQuantity float64             `json:"current_quantity"`
	Status          string              `json:"status"`
	DaysToExpiry    *int                `json:"days_to_expiry,omitempty"`
	ProductName     *string             `json:"product_name,omitempty"`
	Message         string              `json:"message,omitempty"`
}

// UpdateBatchDto - DTO for correcting a lot. Only the two fields a shop mistypes
// off the supplier's label are editable (INV-014 D3): unit_cost is booked COGS
// and current_quantity is owned by consumption. Both fields are pointers so an
// omitted one means "leave it alone" rather than "clear it".
type UpdateBatchDto struct {
	LotNumber  *string              `json:"lot_number" binding:"omitempty,min=1,max=100"`
	ExpiryDate *coreModels.DateOnly `json:"expiry_date" binding:"omitempty"`
}

// RedistributeBatchTarget - one combination and the amount of the lot moving to
// it.
type RedistributeBatchTarget struct {
	SkuID    string  `json:"sku_id" binding:"required,uuid"`
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
}

// RedistributeBatchesDto - splits one default-bucket lot across real
// combinations (INV-014 D2). The lot is never reassigned: the original shrinks
// and the children carry the same lot number, dates and unit cost, so no row
// that has been consumed ever changes its sku_id.
type RedistributeBatchesDto struct {
	BatchID string                    `json:"batch_id" binding:"required,uuid"`
	Targets []RedistributeBatchTarget `json:"targets" binding:"required,min=1,dive"`
}

// RedistributeBatchesResponse - what the split produced.
type RedistributeBatchesResponse struct {
	BatchID       uuid.UUID       `json:"batch_id"`
	MovedQuantity float64         `json:"moved_quantity"`
	TargetCount   int             `json:"target_count"`
	Batches       []BatchResponse `json:"batches"`
	Message       string          `json:"message"`
}

// Batch - Domain model
type Batch struct {
	ID              uuid.UUID
	ProductID       uuid.UUID
	LotNumber       string
	PurchaseDate    time.Time
	ExpiryDate      time.Time
	UnitCost        float64
	InitialQuantity float64
	CurrentQuantity float64
	Status          string // active, expiring_soon, expired, depleted
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListBatchesResponse - Response for listing batches
type ListBatchesResponse struct {
	Batches []BatchResponse `json:"batches"`
	Count   int             `json:"count"`
	Message string          `json:"message"`
}
