package controllers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/limits"
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

// CreateProductWithVariantsInput is CreateProductWithVariants's request.
type CreateProductWithVariantsInput struct {
	JSON api.Body[models.CreateProductDto]
}

// CreateProductWithVariants is POST /products: Create a product with variants.
func (ctrl *ProductController) CreateProductWithVariants(ctx context.Context, in *CreateProductWithVariantsInput) (*api.Out[*models.CreateProductResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateProductDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	product, err := ctrl.productService.CreateProductWithVariants(c.Request.Context(), tenantID, dto)
	if err != nil {
		if err := limits.Refuse(c, err); err != nil {
			return nil, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductSkuAlreadyExists:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductSkuAlreadyExists))
			// The variant tree can now fail on create for the same reasons it
			// fails on update; before INV-012 all three arrived here as a 500.
			case inventoryErrors.ProductVariantGroupDuplicateType:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupDuplicateType))
			case inventoryErrors.ProductVariantOptionDuplicateName:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionDuplicateName))
			case inventoryErrors.ProductVariantGroupAxisNotSingleChoice:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupAxisNotSingleChoice))
			// INV-012 D2 — the create SP generates the combinations of a product
			// that declares an axis, so what the generator refuses now refuses
			// the creation itself.
			case inventoryErrors.SkusCapExceeded:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusCapExceeded))
			case inventoryErrors.SkusSkuCollision:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusSkuCollision))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(product)
}

// UpdateProductWithVariantsInput is UpdateProductWithVariants's request.
type UpdateProductWithVariantsInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateProductDto]
}

// UpdateProductWithVariants is PUT /products/{id}: Update a product and its variants.
func (ctrl *ProductController) UpdateProductWithVariants(ctx context.Context, in *UpdateProductWithVariantsInput) (*api.Out[*models.UpdateProductResponse], error) {
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

	var dto models.UpdateProductDto

	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	product, err := ctrl.productService.UpdateProductWithVariants(c.Request.Context(), tenantID, productID, dto)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			case inventoryErrors.ProductVariantGroupDuplicateType:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupDuplicateType))
			case inventoryErrors.ProductVariantOptionDuplicateName:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionDuplicateName))
			case inventoryErrors.ProductVariantGroupNotInProduct:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupNotInProduct))
			case inventoryErrors.ProductVariantOptionNotInGroup:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantOptionNotInGroup))
			// INV-008 D3 — an axis edit that would take stock or history down
			// with it. The SP refuses instead of cascading.
			case inventoryErrors.SkusAxisHasStock:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusAxisHasStock))
			case inventoryErrors.SkusAxisHasHistory:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, inventoryErrors.SkusAxisHasHistory))
			case inventoryErrors.ProductVariantGroupAxisNotSingleChoice:
				return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductVariantGroupAxisNotSingleChoice))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(product)
}

// GetProductInput is GetProduct's request.
type GetProductInput struct {
	ID string `path:"id"`
}

// GetProduct is GET /products/{id}: Get product by ID.
func (ctrl *ProductController) GetProduct(ctx context.Context, in *GetProductInput) (*api.Out[*models.ProductDetail], error) {
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

	product, err := ctrl.productService.GetProduct(c.Request.Context(), tenantID, productID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(product)
}

// GetProductBySkUInput is GetProductBySkU's request.
type GetProductBySkUInput struct {
	SKU string `path:"sku"`
}

// GetProductBySkU is GET /products/sku/{sku}: Get product by SKU.
func (ctrl *ProductController) GetProductBySkU(ctx context.Context, in *GetProductBySkUInput) (*api.Out[*models.ProductDetail], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	sku := in.SKU

	product, err := ctrl.productService.GetProductBySkU(c.Request.Context(), tenantID, sku)
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

	return api.OK(product)
}

// AddProductMediaInput is AddProductMedia's request.
type AddProductMediaInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.AddProductMediaDto]
}

// AddProductMedia is POST /products/{id}/media: Add media to a product or variant option.
func (ctrl *ProductController) AddProductMedia(ctx context.Context, in *AddProductMediaInput) (*api.Out[*models.AddProductMediaResponse], error) {
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

	var dto models.AddProductMediaDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}

	result, err := ctrl.productService.AddProductMedia(c.Request.Context(), tenantID, productID, dto)
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

	return api.OK(result)
}

// RemoveProductMediaInput is RemoveProductMedia's request.
type RemoveProductMediaInput struct {
	ID      string `path:"id"`
	MediaID string `path:"media_id"`
}

// RemoveProductMediaResponse is RemoveProductMedia's answer.
type RemoveProductMediaResponse struct {
	Message string `json:"message"`
}

// RemoveProductMedia is DELETE /products/{id}/media/{media_id}: Remove a media item from a product.
func (ctrl *ProductController) RemoveProductMedia(ctx context.Context, in *RemoveProductMediaInput) (*api.Out[RemoveProductMediaResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	mediaID := in.MediaID

	if err := ctrl.productService.RemoveProductMedia(c.Request.Context(), tenantID, mediaID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case inventoryErrors.ProductNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, inventoryErrors.ProductNotFound))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(RemoveProductMediaResponse{Message: "Media removed successfully"})
}

// ListProductsInput is ListProducts's request.
type ListProductsInput struct {
	Query api.Query[models.ListProductsQuery]
}

// ListProducts is GET /products: List all products.
func (ctrl *ProductController) ListProducts(ctx context.Context, in *ListProductsInput) (*api.Out[*models.ListProductsResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var query models.ListProductsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Search = strings.TrimSpace(query.Search)

	result, err := ctrl.productService.ListProducts(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(result)
}
