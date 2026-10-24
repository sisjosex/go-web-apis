package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
)

type CategoryController struct {
	categoryService inventoryInterfaces.CategoryService
}

func NewCategoryController(categoryService inventoryInterfaces.CategoryService) *CategoryController {
	return &CategoryController{
		categoryService: categoryService,
	}
}

// CreateCategoryInput is CreateCategory's request.
type CreateCategoryInput struct {
	JSON api.Body[models.CreateCategoryDto]
}

// CreateCategory is POST /categories: Create a new product category.
func (ctrl *CategoryController) CreateCategory(ctx context.Context, in *CreateCategoryInput) (*api.Out[*models.CategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateCategoryDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	category, err := ctrl.categoryService.CreateCategory(c.Request.Context(), tenantID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategorySlugAlreadyExists:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.CategorySlugAlreadyExists))
			case inventoryErrors.CategoryParentNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryParentNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	resp := &models.CategoryResponse{
		ID:           category.ID.String(),
		ParentID:     uuidPtrToStringPtr(category.ParentID),
		Name:         category.Name,
		Slug:         category.Slug,
		Description:  category.Description,
		IconURL:      category.IconURL,
		DisplayOrder: category.DisplayOrder,
		IsActive:     category.IsActive,
		ProductCount: category.ProductCount,
		CreatedAt:    category.CreatedAt.String(),
		UpdatedAt:    category.UpdatedAt.String(),
	}

	return api.OK(resp)
}

// UpdateCategoryInput is UpdateCategory's request.
type UpdateCategoryInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateCategoryDto]
}

// UpdateCategory is PUT /categories/{id}: Update a product category.
func (ctrl *CategoryController) UpdateCategory(ctx context.Context, in *UpdateCategoryInput) (*api.Out[*models.CategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	categoryID := in.ID
	var dto models.UpdateCategoryDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	category, err := ctrl.categoryService.UpdateCategory(c.Request.Context(), tenantID, categoryID, &dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			case inventoryErrors.CategorySlugAlreadyExists:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.CategorySlugAlreadyExists))
			case inventoryErrors.CategoryCircularHierarchy:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryCircularHierarchy))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	resp := &models.CategoryResponse{
		ID:           category.ID.String(),
		ParentID:     uuidPtrToStringPtr(category.ParentID),
		Name:         category.Name,
		Slug:         category.Slug,
		Description:  category.Description,
		IconURL:      category.IconURL,
		DisplayOrder: category.DisplayOrder,
		IsActive:     category.IsActive,
		ProductCount: category.ProductCount,
		CreatedAt:    category.CreatedAt.String(),
		UpdatedAt:    category.UpdatedAt.String(),
	}

	return api.OK(resp)
}

// DeleteCategoryInput is DeleteCategory's request.
type DeleteCategoryInput struct {
	ID string `path:"id"`
}

// DeleteCategoryResponse is DeleteCategory's answer.
type DeleteCategoryResponse struct {
	Message string `json:"message"`
}

// DeleteCategory is DELETE /categories/{id}: Delete a product category.
func (ctrl *CategoryController) DeleteCategory(ctx context.Context, in *DeleteCategoryInput) (*api.Out[DeleteCategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	categoryID := in.ID

	_, err = ctrl.categoryService.DeleteCategory(c.Request.Context(), tenantID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			case inventoryErrors.CategoryHasProducts:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryHasProducts))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(DeleteCategoryResponse{Message: "Category deleted successfully"})
}

// GetCategoryInput is GetCategory's request.
type GetCategoryInput struct {
	ID string `path:"id"`
}

// GetCategory is GET /categories/{id}: Get a product category.
func (ctrl *CategoryController) GetCategory(ctx context.Context, in *GetCategoryInput) (*api.Out[*models.CategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	categoryID := in.ID

	category, err := ctrl.categoryService.GetCategory(c.Request.Context(), tenantID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	resp := &models.CategoryResponse{
		ID:           category.ID.String(),
		ParentID:     uuidPtrToStringPtr(category.ParentID),
		Name:         category.Name,
		Slug:         category.Slug,
		Description:  category.Description,
		IconURL:      category.IconURL,
		DisplayOrder: category.DisplayOrder,
		IsActive:     category.IsActive,
		ProductCount: category.ProductCount,
		CreatedAt:    category.CreatedAt.String(),
		UpdatedAt:    category.UpdatedAt.String(),
	}

	return api.OK(resp)
}

// ListCategoriesInput is ListCategories's request.
type ListCategoriesInput struct {
	ParentID string `query:"parent_id"`
	IsActive string `query:"is_active"`
	Page     string `query:"page"`
	Limit    string `query:"limit"`
}

// ListCategories is GET /categories: List product categories.
func (ctrl *CategoryController) ListCategories(ctx context.Context, in *ListCategoriesInput) (*api.Out[models.ListCategoriesResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	parentID := in.ParentID
	isActive := true
	if isActiveStr := in.IsActive; isActiveStr != "" {
		isActive = isActiveStr == "true"
	}

	page := 1
	if pageStr := in.Page; pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := in.Limit; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	var parentIDPtr *string
	if parentID != "" {
		parentIDPtr = &parentID
	}

	categories, err := ctrl.categoryService.ListCategories(c.Request.Context(), tenantID, parentIDPtr, isActive, limit, offset)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	respCategories := make([]models.CategoryResponse, len(categories))
	for i, cat := range categories {
		respCategories[i] = models.CategoryResponse{
			ID:           cat.ID.String(),
			ParentID:     uuidPtrToStringPtr(cat.ParentID),
			Name:         cat.Name,
			Slug:         cat.Slug,
			Description:  cat.Description,
			IconURL:      cat.IconURL,
			DisplayOrder: cat.DisplayOrder,
			IsActive:     cat.IsActive,
			ProductCount: cat.ProductCount,
			CreatedAt:    cat.CreatedAt.String(),
		}
	}

	return api.OK(models.ListCategoriesResponse{
		Categories: respCategories,
		Total:      int64(len(respCategories)),
	})
}

// SearchCategoriesInput is SearchCategories's request.
type SearchCategoriesInput struct {
	SearchTerm string `query:"search_term"`
	IsActive   string `query:"is_active"`
	Limit      string `query:"limit"`
}

// SearchCategories is GET /categories/search: Search product categories.
func (ctrl *CategoryController) SearchCategories(ctx context.Context, in *SearchCategoriesInput) (*api.Out[[]models.CategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	searchTerm := in.SearchTerm
	if searchTerm == "" {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "search_term is required"))
	}

	var isActive *bool
	if isActiveStr := in.IsActive; isActiveStr != "" {
		val := isActiveStr == "true"
		isActive = &val
	}

	limit := 50
	if limitStr := in.Limit; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	categories, err := ctrl.categoryService.SearchCategories(c.Request.Context(), tenantID, searchTerm, isActive, limit)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	respCategories := make([]models.CategoryResponse, len(categories))
	for i, cat := range categories {
		respCategories[i] = models.CategoryResponse{
			ID:           cat.ID.String(),
			ParentID:     uuidPtrToStringPtr(cat.ParentID),
			Name:         cat.Name,
			Slug:         cat.Slug,
			Description:  cat.Description,
			IconURL:      cat.IconURL,
			DisplayOrder: cat.DisplayOrder,
			IsActive:     cat.IsActive,
			ProductCount: cat.ProductCount,
		}
	}

	return api.OK(respCategories)
}

// AssignProductToCategoryInput is AssignProductToCategory's request.
type AssignProductToCategoryInput struct {
	CategoryID string `path:"category_id"`
	ID         string `path:"id"`
}

// AssignProductToCategoryResponse is AssignProductToCategory's answer.
type AssignProductToCategoryResponse struct {
	Message string `json:"message"`
}

// AssignProductToCategory is POST /products/{id}/categories/{category_id}: Assign product to category.
func (ctrl *CategoryController) AssignProductToCategory(ctx context.Context, in *AssignProductToCategoryInput) (*api.Out[AssignProductToCategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID := in.ID
	categoryID := in.CategoryID

	err = ctrl.categoryService.AssignProductToCategory(c.Request.Context(), tenantID, productID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			case inventoryErrors.ProductAlreadyInCategory:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductAlreadyInCategory))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(AssignProductToCategoryResponse{Message: "Product assigned to category successfully"})
}

// RemoveProductFromCategoryInput is RemoveProductFromCategory's request.
type RemoveProductFromCategoryInput struct {
	CategoryID string `path:"category_id"`
	ID         string `path:"id"`
}

// RemoveProductFromCategoryResponse is RemoveProductFromCategory's answer.
type RemoveProductFromCategoryResponse struct {
	Message string `json:"message"`
}

// RemoveProductFromCategory is DELETE /products/{id}/categories/{category_id}: Remove product from category.
func (ctrl *CategoryController) RemoveProductFromCategory(ctx context.Context, in *RemoveProductFromCategoryInput) (*api.Out[RemoveProductFromCategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID := in.ID
	categoryID := in.CategoryID

	err = ctrl.categoryService.RemoveProductFromCategory(c.Request.Context(), tenantID, productID, categoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(RemoveProductFromCategoryResponse{Message: "Product removed from category successfully"})
}

// GetProductsByCategoryInput is GetProductsByCategory's request.
type GetProductsByCategoryInput struct {
	ID    string `path:"id"`
	Page  string `query:"page"`
	Limit string `query:"limit"`
}

// GetProductsByCategory is GET /categories/{id}/products: Get products in category.
func (ctrl *CategoryController) GetProductsByCategory(ctx context.Context, in *GetProductsByCategoryInput) (*api.Out[[]models.GetProductsByCategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	categoryID := in.ID

	page := 1
	if pageStr := in.Page; pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := in.Limit; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	products, err := ctrl.categoryService.GetProductsByCategory(c.Request.Context(), tenantID, categoryID, limit, offset)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.CategoryNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.CategoryNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	// Ensure we return an empty array instead of null
	if products == nil {
		products = []models.GetProductsByCategoryResponse{}
	}
	return api.OK(products)
}

// GetCategoriesByProductInput is GetCategoriesByProduct's request.
type GetCategoriesByProductInput struct {
	ID    string `path:"id"`
	Page  string `query:"page"`
	Limit string `query:"limit"`
}

// GetCategoriesByProduct is GET /products/{id}/categories: Get categories of a product.
func (ctrl *CategoryController) GetCategoriesByProduct(ctx context.Context, in *GetCategoriesByProductInput) (*api.Out[[]models.CategoryResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	productID := in.ID

	page := 1
	if pageStr := in.Page; pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 100
	if limitStr := in.Limit; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	categories, err := ctrl.categoryService.GetCategoriesByProduct(c.Request.Context(), tenantID, productID, limit, offset)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	respCategories := make([]models.CategoryResponse, len(categories))
	for i, cat := range categories {
		respCategories[i] = models.CategoryResponse{
			ID:           cat.ID.String(),
			ParentID:     uuidPtrToStringPtr(cat.ParentID),
			Name:         cat.Name,
			Slug:         cat.Slug,
			Description:  cat.Description,
			IconURL:      cat.IconURL,
			DisplayOrder: cat.DisplayOrder,
			IsActive:     cat.IsActive,
			ProductCount: cat.ProductCount,
		}
	}

	return api.OK(respCategories)
}

// Helper function to convert optional UUID pointer to string pointer
func uuidPtrToStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	str := id.String()
	if str != "" && str != "00000000-0000-0000-0000-000000000000" {
		return &str
	}
	return nil
}
