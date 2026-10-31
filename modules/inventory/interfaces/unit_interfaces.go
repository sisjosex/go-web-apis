package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// UnitRepository reads and writes units, presentations and codes.
type UnitRepository interface {
	ListUnits(ctx context.Context, tenantID uuid.UUID) ([]models.Unit, error)
	ReplaceProductUnits(ctx context.Context, tenantID uuid.UUID, productID string, dto models.ReplaceProductUnitsDto) error
	FindByCode(ctx context.Context, tenantID uuid.UUID, code string) (*models.CodeLookup, error)
}
