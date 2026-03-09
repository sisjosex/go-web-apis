package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ===== CATEGORY DOMAIN MODEL =====

type Category struct {
	ID           uuid.UUID    `db:"id"`
	ParentID     *uuid.UUID   `db:"parent_id"`
	Name         string       `db:"name"`
	Slug         string       `db:"slug"`
	Description  *string      `db:"description"`
	IconURL      *string      `db:"icon_url"`
	DisplayOrder int          `db:"display_order"`
	IsActive     bool         `db:"is_active"`
	ProductCount int64        `db:"product_count"`
	CreatedAt    time.Time    `db:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"`
	DeletedAt    sql.NullTime `db:"deleted_at"`
}

// ===== CATEGORY DTOs =====

// CreateCategoryDto - Request DTO for creating a category
type CreateCategoryDto struct {
	Name         string  `json:"name" binding:"required"`
	Slug         string  `json:"slug" binding:"required"`
	ParentID     *string `json:"parent_id" binding:"omitempty,uuidv4"`
	Description  *string `json:"description"`
	IconURL      *string `json:"icon_url"`
	DisplayOrder *int    `json:"display_order"`
}

// UpdateCategoryDto - Request DTO for updating a category
type UpdateCategoryDto struct {
	Name         *string `json:"name"`
	Slug         *string `json:"slug"`
	ParentID     *string `json:"parent_id" binding:"omitempty,uuidv4"`
	Description  *string `json:"description"`
	IconURL      *string `json:"icon_url"`
	DisplayOrder *int    `json:"display_order"`
	IsActive     *bool   `json:"is_active"`
}

// CategoryResponse - Response DTO for category
type CategoryResponse struct {
	ID           string  `json:"id"`
	ParentID     *string `json:"parent_id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description"`
	IconURL      *string `json:"icon_url"`
	DisplayOrder int     `json:"display_order"`
	IsActive     bool    `json:"is_active"`
	ProductCount int64   `json:"product_count"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// CategoryTreeResponse - Response DTO for category with children
type CategoryTreeResponse struct {
	ID           string                 `json:"id"`
	ParentID     *string                `json:"parent_id"`
	Name         string                 `json:"name"`
	Slug         string                 `json:"slug"`
	Description  *string                `json:"description"`
	IconURL      *string                `json:"icon_url"`
	DisplayOrder int                    `json:"display_order"`
	IsActive     bool                   `json:"is_active"`
	ProductCount int64                  `json:"product_count"`
	Children     []CategoryTreeResponse `json:"children,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

// AssignProductToCategoryDto - Request DTO for assigning product to category
type AssignProductToCategoryDto struct {
	CategoryID string `json:"category_id" binding:"required,uuidv4"`
}

// GetProductsByCategoryResponse - Response DTO for products in category
type GetProductsByCategoryResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	SKU         string  `json:"sku"`
	Description *string `json:"description"`
	BasePrice   float64 `json:"base_price"`
	Status      string  `json:"status"`
}

// ListCategoriesResponse - Response DTO for list of categories
type ListCategoriesResponse struct {
	Categories []CategoryResponse `json:"categories"`
	Total      int64              `json:"total"`
}

// SearchCategoriesDto - Request DTO for searching categories
type SearchCategoriesDto struct {
	SearchTerm string `json:"search_term" binding:"required"`
	IsActive   *bool  `json:"is_active"`
	Limit      *int   `json:"limit"`
}
