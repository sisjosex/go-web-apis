package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	coreErrors "josex/web/modules/core/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	inventoryInterfaces "josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
)

type ProductController struct {
	productService inventoryInterfaces.ProductService
}

func NewProductController(productService inventoryInterfaces.ProductService) *ProductController {
	return &ProductController{
		productService: productService,
	}
}

// CreateProductWithVariants godoc
// @Summary Create a product with variants
// @Description Create a new product with optional variants (groups and options)
// @Tags inventory
// @Accept json
// @Produce json
// @Param request body models.CreateProductDto true "Product creation request"
// @Success 201 {object} models.CreateProductResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/products [post]
func (ctrl *ProductController) CreateProductWithVariants(c *gin.Context) {
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

	var dto models.CreateProductDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	product, err := ctrl.productService.CreateProductWithVariants(c.Request.Context(), tenantID, dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductSkuAlreadyExists:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductSkuAlreadyExists))
				return
			// The variant tree can now fail on create for the same reasons it
			// fails on update; before INV-012 all three arrived here as a 500.
			case inventoryErrors.ProductVariantGroupDuplicateType:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupDuplicateType))
				return
			case inventoryErrors.ProductVariantOptionDuplicateName:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionDuplicateName))
				return
			case inventoryErrors.ProductVariantGroupAxisNotSingleChoice:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupAxisNotSingleChoice))
				return
			// INV-012 D2 — the create SP generates the combinations of a product
			// that declares an axis, so what the generator refuses now refuses
			// the creation itself.
			case inventoryErrors.SkusCapExceeded:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusCapExceeded))
				return
			case inventoryErrors.SkusSkuCollision:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusSkuCollision))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, product)
}

// UpdateProductWithVariants godoc
// @Summary Update a product and its variants
// @Description Update name, description and base price, and diff the variant tree by id. Omitting "variants" leaves the tree untouched; {"groups": []} removes every group. The SKU is immutable.
// @Tags inventory
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param request body models.UpdateProductDto true "Product update request"
// @Success 200 {object} models.UpdateProductResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Router /api/v1/inventory/products/:id [put]
func (ctrl *ProductController) UpdateProductWithVariants(c *gin.Context) {
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

	var dto models.UpdateProductDto

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	product, err := ctrl.productService.UpdateProductWithVariants(c.Request.Context(), tenantID, productID, dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			case inventoryErrors.ProductVariantGroupDuplicateType:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupDuplicateType))
				return
			case inventoryErrors.ProductVariantOptionDuplicateName:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionDuplicateName))
				return
			case inventoryErrors.ProductVariantGroupNotInProduct:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupNotInProduct))
				return
			case inventoryErrors.ProductVariantOptionNotInGroup:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionNotInGroup))
				return
			// INV-008 D3 — an axis edit that would take stock or history down
			// with it. The SP refuses instead of cascading.
			case inventoryErrors.SkusAxisHasStock:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusAxisHasStock))
				return
			case inventoryErrors.SkusAxisHasHistory:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusAxisHasHistory))
				return
			case inventoryErrors.ProductVariantGroupAxisNotSingleChoice:
				c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupAxisNotSingleChoice))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, product)
}

// GetProduct godoc
// @Summary Get product by ID
// @Description Retrieve a product with all its details
// @Tags inventory
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} models.ProductDetail
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/products/:id [get]
func (ctrl *ProductController) GetProduct(c *gin.Context) {
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

	product, err := ctrl.productService.GetProduct(c.Request.Context(), tenantID, productID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, product)
}

// GetProductBySkU godoc
// @Summary Get product by SKU
// @Description Retrieve a product by its SKU code
// @Tags inventory
// @Produce json
// @Param sku path string true "Product SKU"
// @Success 200 {object} models.ProductDetail
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/inventory/products/sku/:sku [get]
func (ctrl *ProductController) GetProductBySkU(c *gin.Context) {
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

	sku := c.Param("sku")

	product, err := ctrl.productService.GetProductBySkU(c.Request.Context(), tenantID, sku)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, product)
}

// AddProductMedia godoc
// @Summary Add media to a product or variant option
// @Tags inventory
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param request body models.AddProductMediaDto true "Media data"
// @Success 201 {object} models.AddProductMediaResponse
// @Router /api/v1/inventory/products/:id/media [post]
func (ctrl *ProductController) AddProductMedia(c *gin.Context) {
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

	var dto models.AddProductMediaDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
		return
	}

	result, err := ctrl.productService.AddProductMedia(c.Request.Context(), tenantID, productID, dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, result)
}

// RemoveProductMedia godoc
// @Summary Remove a media item from a product
// @Tags inventory
// @Produce json
// @Param id path string true "Product ID"
// @Param media_id path string true "Media ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/inventory/products/:id/media/:media_id [delete]
func (ctrl *ProductController) RemoveProductMedia(c *gin.Context) {
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

	mediaID := c.Param("media_id")

	if err := ctrl.productService.RemoveProductMedia(c.Request.Context(), tenantID, mediaID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media removed successfully"})
}

// ListProducts godoc
// @Summary List all products
// @Description Retrieve a paginated list of active products, each with the categories it is filed under. Optionally narrowed to one category and/or a name/SKU search.
// @Tags inventory
// @Produce json
// @Param limit query int false "Limit (default 20)"
// @Param offset query int false "Offset (default 0)"
// @Param category_id query string false "Only products assigned to this category"
// @Param search query string false "Case-insensitive match on name or SKU"
// @Success 200 {array} models.Product
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/inventory/products [get]
func (ctrl *ProductController) ListProducts(c *gin.Context) {
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

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// A malformed category_id is a client error — never a silent unfiltered list.
	var categoryID *uuid.UUID
	if raw := c.Query("category_id"); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", "category_id must be a valid UUID"))
			return
		}
		categoryID = &parsed
	}

	var search *string
	if raw := strings.TrimSpace(c.Query("search")); raw != "" {
		search = &raw
	}

	products, err := ctrl.productService.ListProducts(c.Request.Context(), tenantID, limit, offset, categoryID, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if products == nil {
		products = []models.Product{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   products,
		"limit":  limit,
		"offset": offset,
	})
}
