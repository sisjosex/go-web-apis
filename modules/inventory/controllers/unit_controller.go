package controllers

import (
	"context"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UnitController serves the units catalogue, a product's presentations and the code lookup (INV-023).
type UnitController struct {
	units    interfaces.UnitRepository
	products interfaces.ProductService
}

// NewUnitController builds the controller.
func NewUnitController(units interfaces.UnitRepository, products interfaces.ProductService) *UnitController {
	return &UnitController{units: units, products: products}
}

func requestTenant(c *gin.Context) (uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	return tenantID, nil
}

func failUnit(c *gin.Context, err error) error {
	return api.Fail(coreErrors.StatusOf(err), coreErrors.BuildError(c, err))
}

// ListUnits is GET /units: the units a business can use.
func (ctrl *UnitController) ListUnits(ctx context.Context, _ *api.None) (*api.Out[models.ListUnitsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	units, err := ctrl.units.ListUnits(c.Request.Context(), tenantID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.ListUnitsResponse{Units: units})
}

// ReplaceProductUnitsInput is ReplaceProductUnits's request.
type ReplaceProductUnitsInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.ReplaceProductUnitsDto]
}

// ReplaceProductUnits is PUT /products/{id}/units: the whole list, diffed by id; answers the product.
func (ctrl *UnitController) ReplaceProductUnits(ctx context.Context, in *ReplaceProductUnitsInput) (*api.Out[*models.ProductDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.ReplaceProductUnitsDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.units.ReplaceProductUnits(c.Request.Context(), tenantID, in.ID, dto); err != nil {
		return nil, failUnit(c, err)
	}
	product, err := ctrl.products.GetProduct(c.Request.Context(), tenantID, in.ID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(product)
}

// FindByCodeInput is FindByCode's request.
type FindByCodeInput struct {
	Code string `path:"code"`
}

// FindByCode is GET /codes/{code}: what a scanned barcode names.
func (ctrl *UnitController) FindByCode(ctx context.Context, in *FindByCodeInput) (*api.Out[*models.CodeLookup], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	found, err := ctrl.units.FindByCode(c.Request.Context(), tenantID, in.Code)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(found)
}
