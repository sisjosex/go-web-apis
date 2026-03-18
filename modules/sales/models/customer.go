package models

import (
	"time"

	"github.com/google/uuid"
)

// Customer represents a sales customer
type Customer struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	PhoneNumber string    `json:"phone_number"`
	Address     string    `json:"address"`
	City        string    `json:"city"`
	State       string    `json:"state"`
	PostalCode  string    `json:"postal_code"`
	Country     string    `json:"country"`
	Status      string    `json:"status"` // active, inactive, blocked
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateCustomerRequestDto DTO for creating a customer
type CreateCustomerRequestDto struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Email       string `json:"email" binding:"required,email-valid"`
	PhoneNumber string `json:"phone_number" binding:"required,min=7,max=20"`
	Address     string `json:"address" binding:"required,min=5,max=500"`
	City        string `json:"city" binding:"required,min=2,max=100"`
	State       string `json:"state" binding:"required,min=2,max=100"`
	PostalCode  string `json:"postal_code" binding:"required,min=3,max=20"`
	Country     string `json:"country" binding:"required,min=2,max=100"`
}

// UpdateCustomerRequestDto DTO for updating a customer
type UpdateCustomerRequestDto struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=255"`
	PhoneNumber *string `json:"phone_number" binding:"omitempty,min=7,max=20"`
	Address     *string `json:"address" binding:"omitempty,min=5,max=500"`
	City        *string `json:"city" binding:"omitempty,min=2,max=100"`
	State       *string `json:"state" binding:"omitempty,min=2,max=100"`
	PostalCode  *string `json:"postal_code" binding:"omitempty,min=3,max=20"`
	Country     *string `json:"country" binding:"omitempty,min=2,max=100"`
}
