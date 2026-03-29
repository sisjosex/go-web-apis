package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/services"
	"josex/web/modules/inventory/utils"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

type CategoryController struct {
	categoryService *services.CategoryService
}

func NewCategoryController(categoryService *services.CategoryService) *CategoryController {
	return &CategoryController{
		categoryService: categoryService,
	}
}

// CreateCategory godoc
// @Summary Create a new product category
// @Description Create a new product category with hierarchical support
// @Tags inventory-categories
// @Accept json
// @Produce json
// @Param request body models.CreateCategoryDto true "Category creation request"
// @Success 201 {object} models.CategoryResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/categories [post]
func (ctrl *CategoryController) CreateCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	var dto models.CreateCategoryDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	category, err := ctrl.categoryService.CreateCategory(c.Request.Context(), tenantID, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
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

	c.JSON(http.StatusCreated, resp)
}

// UpdateCategory godoc
// @Summary Update a product category
// @Description Update an existing product category
// @Tags inventory-categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param request body models.UpdateCategoryDto true "Category update request"
// @Success 200 {object} models.CategoryResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/categories/:id [put]
func (ctrl *CategoryController) UpdateCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	categoryID := c.Param("id")
	var dto models.UpdateCategoryDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	category, err := ctrl.categoryService.UpdateCategory(c.Request.Context(), tenantID, categoryID, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
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

	c.JSON(http.StatusOK, resp)
}

// DeleteCategory godoc
// @Summary Delete a product category
// @Description Soft delete a product category (cannot have products assigned)
// @Tags inventory-categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/categories/:id [delete]
func (ctrl *CategoryController) DeleteCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	categoryID := c.Param("id")

	_, err = ctrl.categoryService.DeleteCategory(c.Request.Context(), tenantID, categoryID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}

// GetCategory godoc
// @Summary Get a product category
// @Description Retrieve a single product category by ID with product count
// @Tags inventory-categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} models.CategoryResponse
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/categories/:id [get]
func (ctrl *CategoryController) GetCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	categoryID := c.Param("id")

	category, err := ctrl.categoryService.GetCategory(c.Request.Context(), tenantID, categoryID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
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

	c.JSON(http.StatusOK, resp)
}

// ListCategories godoc
// @Summary List product categories
// @Description Retrieve a paginated list of product categories (optionally filtered by parent)
// @Tags inventory-categories
// @Produce json
// @Param parent_id query string false "Parent category ID"
// @Param is_active query bool false "Filter by active status"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} models.ListCategoriesResponse
// @Router /api/v1/inventory/categories [get]
func (ctrl *CategoryController) ListCategories(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	parentID := c.Query("parent_id")
	isActive := true
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive = isActiveStr == "true"
	}

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
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
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
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

	c.JSON(http.StatusOK, models.ListCategoriesResponse{
		Categories: respCategories,
		Total:      int64(len(respCategories)),
	})
}

// SearchCategories godoc
// @Summary Search product categories
// @Description Search categories by name, slug, or description
// @Tags inventory-categories
// @Produce json
// @Param search_term query string true "Search term"
// @Param is_active query bool false "Filter by active status"
// @Param limit query int false "Results limit" default(50)
// @Success 200 {array} models.CategoryResponse
// @Router /api/v1/inventory/categories/search [get]
func (ctrl *CategoryController) SearchCategories(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	searchTerm := c.Query("search_term")
	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "search_term is required"))
		return
	}

	var isActive *bool
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		val := isActiveStr == "true"
		isActive = &val
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	categories, err := ctrl.categoryService.SearchCategories(c.Request.Context(), tenantID, searchTerm, isActive, limit)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
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

	c.JSON(http.StatusOK, respCategories)
}

// AssignProductToCategory godoc
// @Summary Assign product to category
// @Description Assign a product to a category
// @Tags inventory-categories
// @Accept json
// @Produce json
// @Param productId path string true "Product ID"
// @Param id path string true "Category ID"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/products/:productId/categories/:id [post]
func (ctrl *CategoryController) AssignProductToCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	productID := c.Param("id")
	categoryID := c.Param("category_id")

	err = ctrl.categoryService.AssignProductToCategory(c.Request.Context(), tenantID, productID, categoryID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Product assigned to category successfully"})
}

// RemoveProductFromCategory godoc
// @Summary Remove product from category
// @Description Remove a product from a category
// @Tags inventory-categories
// @Produce json
// @Param productId path string true "Product ID"
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/products/:productId/categories/:id [delete]
func (ctrl *CategoryController) RemoveProductFromCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	productID := c.Param("id")
	categoryID := c.Param("category_id")

	err = ctrl.categoryService.RemoveProductFromCategory(c.Request.Context(), tenantID, productID, categoryID)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product removed from category successfully"})
}

// GetProductsByCategory godoc
// @Summary Get products in category
// @Description Retrieve all products assigned to a specific category
// @Tags inventory-categories
// @Produce json
// @Param id path string true "Category ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {array} models.GetProductsByCategoryResponse
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/categories/:id/products [get]
func (ctrl *CategoryController) GetProductsByCategory(c *gin.Context) {
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
		return
	}

	categoryID := c.Param("id")

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	products, err := ctrl.categoryService.GetProductsByCategory(c.Request.Context(), tenantID, categoryID, limit, offset)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	// Ensure we return an empty array instead of null
	if products == nil {
		products = []models.GetProductsByCategoryResponse{}
	}
	c.JSON(http.StatusOK, products)
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

// Error mapping utility for categories
func mapCategoryError(err error) (int, string) {
	errStr := err.Error()
	switch errStr {
	case inventoryErrors.CategoryNotFound:
		return http.StatusNotFound, inventoryErrors.CategoryNotFound
	case inventoryErrors.ProductNotFound:
		return http.StatusNotFound, inventoryErrors.ProductNotFound
	case inventoryErrors.CategorySlugAlreadyExists:
		return http.StatusConflict, inventoryErrors.CategorySlugAlreadyExists
	case inventoryErrors.CategoryCircularHierarchy:
		return http.StatusBadRequest, inventoryErrors.CategoryCircularHierarchy
	case inventoryErrors.CategoryHasProducts:
		return http.StatusBadRequest, inventoryErrors.CategoryHasProducts
	case inventoryErrors.ProductAlreadyInCategory:
		return http.StatusConflict, inventoryErrors.ProductAlreadyInCategory
	default:
		return http.StatusInternalServerError, ""
	}
}
