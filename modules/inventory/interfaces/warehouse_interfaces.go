package interfaces

import (
	"context"
	"time"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// WarehouseRepository reads and writes warehouses, stock across them and transfers (INV-025).
type WarehouseRepository interface {
	ListWarehouses(ctx context.Context, tenantID uuid.UUID, id *string) ([]models.Warehouse, error)
	UpsertWarehouse(ctx context.Context, tenantID uuid.UUID, id *string, dto models.UpsertWarehouseDto) (string, error)
	SetWarehouseStatus(ctx context.Context, tenantID uuid.UUID, id, status string) error
	SetDefaultWarehouse(ctx context.Context, tenantID uuid.UUID, id string) error
	ListStock(ctx context.Context, tenantID uuid.UUID, today time.Time, query models.ListStockQuery) (*models.ListStockResponse, error)
	ListStockSkus(ctx context.Context, tenantID uuid.UUID, productID string) ([]models.StockSku, error)
	Summary(ctx context.Context, tenantID uuid.UUID, today time.Time, warehouseID *string) (*models.InventorySummary, error)
	NavBadges(ctx context.Context, tenantID uuid.UUID) (*models.InventoryNavBadges, error)
	CreateTransfer(ctx context.Context, tenantID uuid.UUID, dto models.CreateTransferDto, by *string) (string, error)
	ShipTransfer(ctx context.Context, tenantID uuid.UUID, id string, note, by *string) error
	ReceiveTransfer(ctx context.Context, tenantID uuid.UUID, id string, lines []models.ReceivedLineDto, by *string) error
	CancelTransfer(ctx context.Context, tenantID uuid.UUID, id string, by *string) error
	GetTransfer(ctx context.Context, tenantID uuid.UUID, id string) (*models.Transfer, error)
	ListTransfers(ctx context.Context, tenantID uuid.UUID, query models.ListTransfersQuery) (*models.ListTransfersResponse, error)
}
