package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// TenantService defines business logic for tenant management
type TenantService interface {
	// Create a new tenant
	CreateTenant(ctx context.Context, dto *models.CreateTenantDto, creatorUserID uuid.UUID) (*models.TenantDetailResponse, error)

	// Get user's accessible tenants
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*models.UserTenantResponse, error)

	// Update tenant (admin/owner only - enforced in controller)
	UpdateTenant(ctx context.Context, tenantID uuid.UUID, dto *models.UpdateTenantDto) (*models.TenantDetailResponse, error)

	// Add user to tenant (requesterUserID validates permissions)
	AddUserToTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, dto *models.AddUserToTenantDto) error

	// Remove user from tenant (requesterUserID validates permissions)
	RemoveUserFromTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID) error

	// Verify user has access to tenant (used by middleware)
	VerifyUserTenantAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, error)

	// Count how many tenants a user owns (for subscription limits)
	CountUserOwnedTenants(ctx context.Context, userID uuid.UUID) (int, error)
}
