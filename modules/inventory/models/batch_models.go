package models

import (
	"time"

	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
)

// CreateBatchDto - DTO for creating a new batch
type CreateBatchDto struct {
	ProductID       string              `json:"product_id" binding:"required,uuidv4"`
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
	LotNumber       string              `json:"lot_number"`
	PurchaseDate    coreModels.DateOnly `json:"purchase_date"`
	ExpiryDate      coreModels.DateOnly `json:"expiry_date"`
	UnitCost        float64             `json:"unit_cost"`
	InitialQuantity float64             `json:"initial_quantity"`
	CurrentQuantity float64             `json:"current_quantity"`
	Status          string              `json:"status"`
	DaysToExpiry    *int                `json:"days_to_expiry,omitempty"`
	Message         string              `json:"message,omitempty"`
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
