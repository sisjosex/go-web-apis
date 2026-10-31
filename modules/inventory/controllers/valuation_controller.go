package controllers

import (
	"context"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/danielgtaylor/huma/v2"
)

// ValuationController serves what the stock is worth, on screen and as a file (INV-022).
type ValuationController struct {
	valuation interfaces.ValuationRepository
}

// NewValuationController builds the controller.
func NewValuationController(valuation interfaces.ValuationRepository) *ValuationController {
	return &ValuationController{valuation: valuation}
}

// ValuationInput is Valuation's request.
type ValuationInput struct {
	Query api.Query[models.ValuationQuery]
}

// Valuation is GET /reports/valuation.
func (ctrl *ValuationController) Valuation(ctx context.Context, in *ValuationInput) (*api.Out[*models.ValuationResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ValuationQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	out, err := ctrl.valuation.Valuation(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// ExportValuation is GET /export/valuation: the same rows, as a file.
func (ctrl *ValuationController) ExportValuation(ctx context.Context, in *ValuationInput) (*huma.StreamResponse, error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ValuationQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.PageSize = exportPageSize
	return streamPages(c, "valuation", []string{"sku", "name", "quantity", "unit_cost", "value"},
		func(ctx context.Context, page int) ([]models.ValuationItem, int64, error) {
			query.Page = page
			out, err := ctrl.valuation.Valuation(ctx, tenantID, query)
			if err != nil {
				return nil, 0, err
			}
			return out.Items, out.TotalCount, nil
		},
		func(row models.ValuationItem) []string {
			return []string{row.SKU, row.Name, number(row.Quantity), optionalNumber(row.UnitCost), optionalNumber(row.Value)}
		})
}

func optionalNumber(value *float64) string {
	if value == nil {
		return ""
	}
	return number(*value)
}
