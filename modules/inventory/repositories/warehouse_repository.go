package repositories

import (
	"context"
	"encoding/json"
	"time"

	"josex/web/modules/core/limits"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// WarehouseRepository calls the warehouse, stock-across-warehouses and transfer SPs.
type WarehouseRepository struct {
	dbService coreServices.DatabaseService
}

// NewWarehouseRepository builds the repository.
func NewWarehouseRepository(dbService coreServices.DatabaseService) *WarehouseRepository {
	return &WarehouseRepository{dbService: dbService}
}

// ListWarehouses answers all of a business's, or the one named.
func (r *WarehouseRepository) ListWarehouses(ctx context.Context, tenantID uuid.UUID, id *string) ([]models.Warehouse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(id AS VARCHAR), name, address, city, kind, CAST(manager_id AS VARCHAR), manager_name, is_default, status, value, units, below_min
		 FROM inventory.sp_list_warehouses($1, $2::UUID)`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Warehouse{}
	for rows.Next() {
		var row models.Warehouse
		var managerID, managerName *string
		if err := rows.Scan(&row.ID, &row.Name, &row.Address, &row.City, &row.Kind, &managerID, &managerName, &row.IsDefault, &row.Status, &row.Value, &row.Units, &row.BelowMin); err != nil {
			return nil, err
		}
		if managerID != nil && managerName != nil {
			row.Manager = &models.WarehouseManager{ID: *managerID, Name: *managerName}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UpsertWarehouse creates within the plan's limit (D5), or changes the one named.
func (r *WarehouseRepository) UpsertWarehouse(ctx context.Context, tenantID uuid.UUID, id *string, dto models.UpsertWarehouseDto) (string, error) {
	var out string
	if id != nil {
		err := r.dbService.QueryRow(ctx,
			`SELECT CAST(inventory.sp_upsert_warehouse($1, $2::UUID, $3, $4, $5, NULLIF($6, ''), $7::UUID) AS VARCHAR)`,
			tenantID, *id, dto.Name, dto.Address, dto.City, dto.Kind, dto.ManagerUserID).Scan(&out)
		return out, err
	}
	err := r.dbService.QueryRow(ctx,
		`SELECT CAST(inventory.sp_upsert_warehouse(lim.tenant_id, NULL, $2, $3, $4, NULLIF($5, ''), $6::UUID) AS VARCHAR)
		 FROM public.fn_within_limit($1, (
			SELECT COUNT(*) FROM inventory.warehouses x WHERE x.tenant_id = $1 AND x.status = 'active'
		 ), $7::INT, 'inventory_warehouses') lim(tenant_id)`,
		tenantID, dto.Name, dto.Address, dto.City, dto.Kind, dto.ManagerUserID, limits.For(ctx, tenantID, limits.InventoryWarehouses)).Scan(&out)
	return out, err
}

// SetWarehouseStatus archives or reactivates it.
func (r *WarehouseRepository) SetWarehouseStatus(ctx context.Context, tenantID uuid.UUID, id, status string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_set_warehouse_status($1, $2::UUID, $3)`, tenantID, id, status)
	return err
}

// SetDefaultWarehouse moves the default mark.
func (r *WarehouseRepository) SetDefaultWarehouse(ctx context.Context, tenantID uuid.UUID, id string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_set_default_warehouse($1, $2::UUID)`, tenantID, id)
	return err
}

// ListStock answers a page of the stock screen.
func (r *WarehouseRepository) ListStock(ctx context.Context, tenantID uuid.UUID, today time.Time, query models.ListStockQuery) (*models.ListStockResponse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(product_id AS VARCHAR), name, sku, variants_count, low_variants, by_warehouse, total, reorder_level, status,
		        CAST(short_warehouse_id AS VARCHAR), next_lot, counts, total_count
		 FROM inventory.sp_list_stock($1, $2, $3::UUID, $4, $5::UUID, $6, $7, $8)`,
		tenantID, today, query.WarehouseID, query.Search, query.CategoryID, query.Filter, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &models.ListStockResponse{Items: []models.StockItem{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		var id, name, sku, status *string
		var variants, low *int
		var total, reorder *float64
		var byWarehouse, nextLot, counts []byte
		var item models.StockItem
		if err := rows.Scan(&id, &name, &sku, &variants, &low, &byWarehouse, &total, &reorder, &status, &item.ShortWarehouseID, &nextLot, &counts, &out.TotalCount); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(counts, &out.Counts); err != nil {
			return nil, err
		}
		if id == nil {
			continue
		}
		item.ProductID, item.Name, item.SKU, item.Status = *id, *name, *sku, *status
		item.VariantsCount, item.LowVariants, item.Total, item.ReorderLevel = *variants, *low, *total, *reorder
		if err := json.Unmarshal(byWarehouse, &item.ByWarehouse); err != nil {
			return nil, err
		}
		if nextLot != nil {
			if err := json.Unmarshal(nextLot, &item.NextLot); err != nil {
				return nil, err
			}
		}
		out.Items = append(out.Items, item)
	}
	return out, rows.Err()
}

// ListStockSkus answers a product's combinations per warehouse.
func (r *WarehouseRepository) ListStockSkus(ctx context.Context, tenantID uuid.UUID, productID string) ([]models.StockSku, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(sku_id AS VARCHAR), label, by_warehouse, total, reorder_level FROM inventory.sp_list_stock_skus($1, $2::UUID)`,
		tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.StockSku{}
	for rows.Next() {
		var row models.StockSku
		var byWarehouse []byte
		if err := rows.Scan(&row.SkuID, &row.Label, &byWarehouse, &row.Total, &row.ReorderLevel); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(byWarehouse, &row.ByWarehouse); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Summary answers the inventory home.
func (r *WarehouseRepository) Summary(ctx context.Context, tenantID uuid.UUID, today time.Time, warehouseID *string) (*models.InventorySummary, error) {
	var body []byte
	if err := r.dbService.QueryRow(ctx, `SELECT inventory.sp_inventory_summary($1, $2, $3::UUID)`, tenantID, today, warehouseID).Scan(&body); err != nil {
		return nil, err
	}
	var out models.InventorySummary
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NavBadges answers the numbers beside the menu.
func (r *WarehouseRepository) NavBadges(ctx context.Context, tenantID uuid.UUID) (*models.InventoryNavBadges, error) {
	var out models.InventoryNavBadges
	err := r.dbService.QueryRow(ctx, `SELECT stock_low FROM inventory.sp_inventory_nav_badges($1)`, tenantID).Scan(&out.StockLow)
	return &out, err
}

// CreateTransfer prepares a transfer and answers its id.
func (r *WarehouseRepository) CreateTransfer(ctx context.Context, tenantID uuid.UUID, dto models.CreateTransferDto, by *string) (string, error) {
	lines, err := json.Marshal(dto.Lines)
	if err != nil {
		return "", err
	}
	var id string
	err = r.dbService.QueryRow(ctx,
		`SELECT CAST(inventory.sp_create_transfer($1, $2::UUID, $3::UUID, $4::JSONB, $5, $6::UUID) AS VARCHAR)`,
		tenantID, dto.FromWarehouseID, dto.ToWarehouseID, string(lines), dto.Notes, by).Scan(&id)
	return id, err
}

// ShipTransfer marks it in transit.
func (r *WarehouseRepository) ShipTransfer(ctx context.Context, tenantID uuid.UUID, id string, note, by *string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_ship_transfer($1, $2::UUID, $3, $4::UUID)`, tenantID, id, note, by)
	return err
}

// ReceiveTransfer receives it at its destination.
func (r *WarehouseRepository) ReceiveTransfer(ctx context.Context, tenantID uuid.UUID, id string, lines []models.ReceivedLineDto, by *string) error {
	body, err := json.Marshal(lines)
	if err != nil {
		return err
	}
	_, err = r.dbService.Execute(ctx, `SELECT inventory.sp_receive_transfer($1, $2::UUID, $3::JSONB, $4::UUID)`, tenantID, id, string(body), by)
	return err
}

// CancelTransfer cancels it while prepared.
func (r *WarehouseRepository) CancelTransfer(ctx context.Context, tenantID uuid.UUID, id string, by *string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_cancel_transfer($1, $2::UUID, $3::UUID)`, tenantID, id, by)
	return err
}

// GetTransfer answers it with its lines and timeline.
func (r *WarehouseRepository) GetTransfer(ctx context.Context, tenantID uuid.UUID, id string) (*models.Transfer, error) {
	var body []byte
	if err := r.dbService.QueryRow(ctx, `SELECT inventory.sp_get_transfer($1, $2::UUID)`, tenantID, id).Scan(&body); err != nil {
		return nil, err
	}
	var out models.Transfer
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListTransfers answers a page.
func (r *WarehouseRepository) ListTransfers(ctx context.Context, tenantID uuid.UUID, query models.ListTransfersQuery) (*models.ListTransfersResponse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT CAST(id AS VARCHAR), number, status, from_name, to_name, units, prepared_at, total_count
		 FROM inventory.sp_list_transfers($1, $2, $3, $4)`, tenantID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &models.ListTransfersResponse{Transfers: []models.TransferRow{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		var row models.TransferRow
		if err := rows.Scan(&row.ID, &row.Number, &row.Status, &row.FromName, &row.ToName, &row.Units, &row.PreparedAt, &out.TotalCount); err != nil {
			return nil, err
		}
		out.Transfers = append(out.Transfers, row)
	}
	return out, rows.Err()
}
