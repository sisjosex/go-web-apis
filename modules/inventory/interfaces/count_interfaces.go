package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// CountRepository reads and writes stock counts (INV-026).
type CountRepository interface {
	CreateCount(ctx context.Context, tenantID uuid.UUID, dto models.CreateCountDto, by *string) (string, error)
	ListCounts(ctx context.Context, tenantID uuid.UUID, query models.ListCountsQuery) (*models.ListCountsResponse, error)
	GetCount(ctx context.Context, tenantID uuid.UUID, id string) (*models.CountDetail, error)
	SetCountLines(ctx context.Context, tenantID uuid.UUID, id string, lines []models.CountLineDto) error
	ScanCount(ctx context.Context, tenantID uuid.UUID, id, code string) error
	SetCountStatus(ctx context.Context, tenantID uuid.UUID, id, action string) error
	CloseCount(ctx context.Context, tenantID uuid.UUID, id string, by *string) (string, error)
}
