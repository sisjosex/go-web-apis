package interfaces

import (
	"context"

	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// TenantRepository defines methods for tenant data access
type TenantRepository interface {
	// Create a new tenant
	CreateTenant(ctx context.Context, dto *models.CreateTenantDto, creatorUserID uuid.UUID) (*models.Tenant, error)

	// Get tenant by slug
	GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error)

	// Verify user has access to tenant (returns tenant + user role)
	VerifyUserTenantAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, error)

	// Get user's accessible tenants
	GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*models.UserTenantResponse, error)

	// Update tenant
	UpdateTenant(ctx context.Context, tenantID uuid.UUID, dto *models.UpdateTenantDto) (*models.Tenant, error)

	// Add user to tenant (requesterUserID validates permissions)
	AddUserToTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) (*models.TenantUser, error)

	// Remove user from tenant (requesterUserID validates permissions)
	RemoveUserFromTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID) error

	// Count how many tenants a user owns (for subscription limits)
	CountUserOwnedTenants(ctx context.Context, userID uuid.UUID) (int, error)

	// List all tenants with custom database URLs (for CLI migrations)
	ListTenantsWithCustomDB(ctx context.Context) ([]*models.Tenant, error)
	// ListTenantAdminEmails answers who an operational digest goes to (TRACK-016 D1).
	ListTenantAdminEmails(ctx context.Context, tenantID uuid.UUID) ([]*models.TenantAdminEmail, error)

	// Update a user's role within a tenant
	UpdateUserRole(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) error
}
