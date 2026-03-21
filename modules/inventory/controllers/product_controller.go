package controllers

import (
	"net/http"
	"strconv"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/services"
	"josex/web/modules/inventory/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProductController struct {
	productService *services.ProductService
}

func NewProductController(productService *services.ProductService) *ProductController {
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
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, product)
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
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
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
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, product)
}

// ListProducts godoc
// @Summary List all products
// @Description Retrieve a paginated list of active products
// @Tags inventory
// @Produce json
// @Param limit query int false "Limit (default 20)"
// @Param offset query int false "Offset (default 0)"
// @Success 200 {array} models.Product
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

	products, err := ctrl.productService.ListProducts(c.Request.Context(), tenantID, limit, offset)
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
