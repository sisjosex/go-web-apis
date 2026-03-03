package models

import (
	"time"
)

// DTOs for API requests/responses

type CreateProductDto struct {
	SKU         string      `json:"sku" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	Description *string     `json:"description"`
	BasePrice   float64     `json:"base_price" binding:"required,gt=0"`
	Variants    interface{} `json:"variants"`
}

type RecordMovementDto struct {
	ProductID     string   `json:"product_id" binding:"required"`
	MovementType  string   `json:"movement_type" binding:"required"`
	Quantity      float64  `json:"quantity" binding:"required"`
	ReferenceType *string  `json:"reference_type"`
	ReferenceID   *string  `json:"reference_id"`
	UnitCost      *float64 `json:"unit_cost"`
	Notes         *string  `json:"notes"`
}

type CreateProductResponse struct {
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

// Domain Models

type Product struct {
	ID          string    `db:"id" json:"id"`
	SKU         string    `db:"sku" json:"sku"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description"`
	BasePrice   float64   `db:"base_price" json:"base_price"`
	HasVariants bool      `db:"has_variants" json:"has_variants"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

type ProductStock struct {
	ID              string    `db:"id" json:"id"`
	ProductID       string    `db:"product_id" json:"product_id"`
	CurrentQuantity float64   `db:"current_quantity" json:"current_quantity"`
	LastUpdatedAt   time.Time `db:"last_updated_at" json:"last_updated_at"`
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
}
