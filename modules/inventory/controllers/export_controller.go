package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/services/export"
	coreUtils "josex/web/modules/core/utils"
	inventoryErrors "josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
	"josex/web/modules/inventory/utils"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// exportPageSize is what a download reads the list SPs by: memory stays one page (INV-021 D1).
const exportPageSize = 1000

// ExportController writes the catalogue, the movements and the lots as CSV, with the filters of their screens.
// A stream has no body type to document; the contract lists their filters and refusals.
type ExportController struct {
	products  interfaces.ProductService
	movements interfaces.MovementService
	batches   interfaces.BatchService
}

// NewExportController builds the controller.
func NewExportController(products interfaces.ProductService, movements interfaces.MovementService, batches interfaces.BatchService) *ExportController {
	return &ExportController{products: products, movements: movements, batches: batches}
}

// streamPages reads a list page by page into the CSV. The first page says how many rows match, so an
// export past the cap is refused before a byte is sent.
func streamPages[T any](c *gin.Context, resource string, header []string, read func(ctx context.Context, page int) ([]T, int64, error), row func(T) []string) (*huma.StreamResponse, error) {
	lang := c.GetString("lang")
	translated := make([]string, len(header))
	for i, key := range header {
		translated[i] = coreUtils.GetTranslation(lang, "inventory.export."+resource+"."+key)
	}
	ctx := c.Request.Context()
	rows, total, err := read(ctx, 1)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if total > export.MaxRows {
		return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, inventoryErrors.ExportTooLarge))
	}
	filename := fmt.Sprintf("%s-%s.csv", resource, utils.TenantToday(c).Format(time.DateOnly))
	return &huma.StreamResponse{Body: func(huma.Context) {
		page, index := 1, 0
		export.StreamCSV(c, filename, translated, func() ([]string, bool, error) {
			if index == len(rows) {
				if len(rows) < exportPageSize {
					return nil, false, nil
				}
				page++
				index = 0
				if rows, _, err = read(ctx, page); err != nil {
					return nil, false, err
				}
				if len(rows) == 0 {
					return nil, false, nil
				}
			}
			index++
			return row(rows[index-1]), true, nil
		})
	}}, nil
}

func tenantOfRequest(c *gin.Context) (uuid.UUID, error) {
	tenantID, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	return tenantID, nil
}

func number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// ExportProductsInput is ExportProducts's request.
type ExportProductsInput struct {
	Query api.Query[models.ListProductsQuery]
}

// ExportProducts is GET /export/products: the list's search, category, status and chip.
func (ctrl *ExportController) ExportProducts(ctx context.Context, in *ExportProductsInput) (*huma.StreamResponse, error) {
	c := api.Gin(ctx)
	tenantID, err := tenantOfRequest(c)
	if err != nil {
		return nil, err
	}
	var query models.ListProductsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.PageSize = exportPageSize
	return streamPages(c, "products", []string{"sku", "name", "category", "price", "stock", "status"},
		func(ctx context.Context, page int) ([]models.Product, int64, error) {
			query.Page = page
			list, err := ctrl.products.ListProducts(ctx, tenantID, query)
			if err != nil {
				return nil, 0, err
			}
			return list.Products, list.TotalCount, nil
		},
		func(p models.Product) []string {
			names := make([]string, 0, len(p.Categories))
			for _, category := range p.Categories {
				names = append(names, category.Name)
			}
			return []string{p.SKU, p.Name, strings.Join(names, ", "), number(p.BasePrice), number(p.AvailableQuantity), p.Status}
		})
}

// ExportMovementsInput is ExportMovements's request.
type ExportMovementsInput struct {
	Query api.Query[models.ListMovementsQuery]
}

// ExportMovements is GET /export/movements: one line per movement, never folded.
func (ctrl *ExportController) ExportMovements(ctx context.Context, in *ExportMovementsInput) (*huma.StreamResponse, error) {
	c := api.Gin(ctx)
	tenantID, err := tenantOfRequest(c)
	if err != nil {
		return nil, err
	}
	var query models.ListMovementsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	query.Group = "none"
	query.PageSize = exportPageSize
	return streamPages(c, "movements", []string{"date", "type", "product", "variant", "quantity", "document", "notes", "by"},
		func(ctx context.Context, page int) ([]models.InventoryMovement, int64, error) {
			query.Page = page
			list, err := ctrl.movements.ListMovements(ctx, tenantID, query)
			if err != nil {
				return nil, 0, err
			}
			return list.Movements, list.TotalCount, nil
		},
		func(m models.InventoryMovement) []string {
			return []string{
				m.CreatedAt.Format("2006-01-02 15:04"), m.MovementType, m.ProductName, text(m.SkuLabel), number(m.Quantity),
				text(m.ReferenceLabel), text(m.Notes), text(m.CreatedByName),
			}
		})
}

// ExportBatchesInput is ExportBatches's request.
type ExportBatchesInput struct {
	ProductID   string `query:"product_id" doc:"One product's lots"`
	WarningDays string `query:"warning_days" doc:"Without a product, the lots expiring within these days (30)"`
}

// ExportBatches is GET /export/batches: one product's lots, or with warning_days the lots about to expire.
func (ctrl *ExportController) ExportBatches(ctx context.Context, in *ExportBatchesInput) (*huma.StreamResponse, error) {
	c := api.Gin(ctx)
	tenantID, err := tenantOfRequest(c)
	if err != nil {
		return nil, err
	}
	read, err := ctrl.batchReader(in, tenantID, utils.TenantToday(c))
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	return streamPages(c, "batches", []string{"lot", "sku", "product", "purchased", "expires", "cost", "quantity", "status"}, read,
		func(b models.BatchResponse) []string {
			return []string{
				b.LotNumber, text(b.Sku), text(b.ProductName), time.Time(b.PurchaseDate).Format(time.DateOnly), time.Time(b.ExpiryDate).Format(time.DateOnly),
				number(b.UnitCost), number(b.CurrentQuantity), b.Status,
			}
		})
}

func (ctrl *ExportController) batchReader(in *ExportBatchesInput, tenantID uuid.UUID, today time.Time) (func(context.Context, int) ([]models.BatchResponse, int64, error), error) {
	if in.ProductID != "" {
		productID, err := uuid.Parse(in.ProductID)
		if err != nil {
			return nil, err
		}
		onlyActive := false
		return func(ctx context.Context, page int) ([]models.BatchResponse, int64, error) {
			list, err := ctrl.batches.ListBatchesByProduct(ctx, tenantID, today, productID, models.ListBatchesQuery{OnlyActive: &onlyActive, Page: page, PageSize: exportPageSize})
			if err != nil {
				return nil, 0, err
			}
			return list.Batches, list.TotalCount, nil
		}, nil
	}
	days, err := 30, error(nil)
	if in.WarningDays != "" {
		days, err = strconv.Atoi(in.WarningDays)
	}
	if err != nil || days < 1 {
		return nil, errors.New("warning_days must be a positive number")
	}
	return func(ctx context.Context, page int) ([]models.BatchResponse, int64, error) {
		list, err := ctrl.batches.GetExpiringBatches(ctx, tenantID, today, models.ListExpiringBatchesQuery{WarningDays: days, Page: page, PageSize: exportPageSize})
		if err != nil {
			return nil, 0, err
		}
		return list.Batches, list.TotalCount, nil
	}, nil
}
