package interfaces

import (
	"context"

	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// ValuationRepository reads what the stock is worth (INV-022).
type ValuationRepository interface {
	Valuation(ctx context.Context, tenantID uuid.UUID, query models.ValuationQuery) (*models.ValuationResponse, error)
}
