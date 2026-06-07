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

	// Get tenant by slug (used by middleware for super_admin access)
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)

	// Verify user has access to tenant (used by middleware)
	VerifyUserTenantAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, error)

	// Count how many tenants a user owns (for subscription limits)
	CountUserOwnedTenants(ctx context.Context, userID uuid.UUID) (int, error)

	// Run migrations on a tenant's database (admin only)
	RunTenantMigrations(ctx context.Context, tenantSlug string) error

	// GetTenantEnabledModuleCodes returns a map of module codes enabled for a tenant
	GetTenantEnabledModuleCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error)

	// Update a user's role within a tenant
	UpdateUserRole(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) error
}
