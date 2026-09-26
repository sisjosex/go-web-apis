package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// GPSDeviceRepository holds the GPS device credentials (TRACK-010): a token's sha256 answers whose
// tenant and vehicle it posts for.
type GPSDeviceRepository interface {
	// Resolve answers the tenant (as the tenant middleware needs it) and the vehicle of a token hash
	// in tenantID, or nil, nil when no device of that tenant holds it.
	Resolve(ctx context.Context, tenantID uuid.UUID, tokenHash []byte) (*models.TenantAccessInfo, *uuid.UUID, error)
	// Issue stores the vehicle's token hash in the tenant, replacing its previous one.
	Issue(ctx context.Context, tenantID, vehicleID uuid.UUID, tokenHash []byte) error
}
