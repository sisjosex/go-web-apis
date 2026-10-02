package repositories

import (
	"context"
	"errors"

	"josex/web/modules/core/services"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type tenantRepository struct {
	dbService services.DatabaseService
}

// NewTenantRepository creates a new instance of TenantRepository
func NewTenantRepository(dbService services.DatabaseService) interfaces.TenantRepository {
	return &tenantRepository{dbService: dbService}
}

// CreateTenant creates a new tenant
func (r *tenantRepository) CreateTenant(ctx context.Context, dto *models.CreateTenantDto, creatorUserID uuid.UUID) (*models.Tenant, error) {
	tenant := &models.Tenant{}
	query := `
		SELECT * FROM tenancy.sp_create_tenant(
			p_slug := $1,
			p_name := $2,
			p_creator_user_id := $3,
			p_database_url := $4,
			p_schema_name := $5,
			p_settings := $6
		)
	`

	// Default values
	schemaName := "public"
	if dto.SchemaName != nil && *dto.SchemaName != "" {
		schemaName = *dto.SchemaName
	}

	settings := "{}"
	if dto.Settings != nil && *dto.Settings != "" {
		settings = *dto.Settings
	}

	params := []interface{}{
		dto.Slug,
		dto.Name,
		creatorUserID,
		dto.DatabaseURL,
		schemaName,
		settings,
	}

	row := r.dbService.QueryRow(ctx, query, params...)

	err := row.Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.DatabaseURL,
		&tenant.SchemaName,
		&tenant.IsActive,
		&tenant.IsSuspended,
		&tenant.SuspendedReason,
		&tenant.Settings,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return tenant, nil
}

// GetTenantBySlug gets tenant by slug
func (r *tenantRepository) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	tenant := &models.Tenant{}
	query := `
		SELECT * FROM tenancy.sp_get_tenant_by_slug(
			p_slug := $1
		)
	`

	row := r.dbService.QueryRow(ctx, query, slug)

	err := row.Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.DatabaseURL,
		&tenant.SchemaName,
		&tenant.IsActive,
		&tenant.IsSuspended,
		&tenant.CreatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return tenant, nil
}

// VerifyUserTenantAccess verifies user has access to tenant
func (r *tenantRepository) VerifyUserTenantAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, error) {
	accessInfo := &models.TenantAccessInfo{}
	query := `
		SELECT * FROM tenancy.sp_verify_user_tenant_access(
			p_user_id := $1,
			p_tenant_slug := $2
		)
	`

	row := r.dbService.QueryRow(ctx, query, userID, slug)

	var permCodes []string
	err := row.Scan(
		&accessInfo.TenantID,
		&accessInfo.Slug,
		&accessInfo.Name,
		&accessInfo.DatabaseURL,
		&accessInfo.SchemaName,
		&accessInfo.IsActive,
		&accessInfo.IsSuspended,
		&accessInfo.UserRole,
		&accessInfo.UserIsActive,
		&permCodes,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	accessInfo.Permissions = make(map[string]bool, len(permCodes))
	for _, code := range permCodes {
		accessInfo.Permissions[code] = true
	}

	return accessInfo, nil
}

// GetUserTenants gets user's accessible tenants
func (r *tenantRepository) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*models.UserTenantResponse, error) {
	query := `
		SELECT * FROM tenancy.sp_get_user_tenants(
			p_user_id := $1
		)
	`

	rows, err := r.dbService.Query(ctx, query, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	// Initialize with empty slice to return [] instead of null
	tenants := make([]*models.UserTenantResponse, 0)
	for rows.Next() {
		tenant := &models.UserTenantResponse{}
		err := rows.Scan(
			&tenant.TenantID,
			&tenant.Slug,
			&tenant.Name,
			&tenant.IsActive,
			&tenant.IsSuspended,
			&tenant.UserRole,
			&tenant.JoinedAt,
		)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}

// UpdateTenant updates tenant information
func (r *tenantRepository) UpdateTenant(ctx context.Context, tenantID uuid.UUID, dto *models.UpdateTenantDto) (*models.Tenant, error) {
	tenant := &models.Tenant{}
	query := `
		SELECT * FROM tenancy.sp_update_tenant(
			p_tenant_id := $1,
			p_name := $2,
			p_database_url := $3,
			p_schema_name := $4,
			p_settings := $5
		)
	`

	params := []interface{}{
		tenantID,
		dto.Name,
		dto.DatabaseURL,
		dto.SchemaName,
		dto.Settings,
	}

	row := r.dbService.QueryRow(ctx, query, params...)

	err := row.Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.DatabaseURL,
		&tenant.SchemaName,
		&tenant.IsActive,
		&tenant.IsSuspended,
		&tenant.SuspendedReason,
		&tenant.Settings,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return tenant, nil
}

// AddUserToTenant adds a user to a tenant
func (r *tenantRepository) AddUserToTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) (*models.TenantUser, error) {
	tenantUser := &models.TenantUser{}
	query := `
		SELECT * FROM tenancy.sp_add_user_to_tenant(
			p_tenant_id := $1,
			p_requester_user_id := $2,
			p_user_id := $3,
			p_role := $4
		)
	`

	row := r.dbService.QueryRow(ctx, query, tenantID, requesterUserID, userID, role)

	err := row.Scan(
		&tenantUser.ID,
		&tenantUser.TenantID,
		&tenantUser.UserID,
		&tenantUser.Role,
		&tenantUser.IsActive,
		&tenantUser.JoinedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return tenantUser, nil
}

// RemoveUserFromTenant removes a user from a tenant
func (r *tenantRepository) RemoveUserFromTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID) error {
	query := `
		SELECT tenancy.sp_remove_user_from_tenant(
			p_tenant_id := $1,
			p_requester_user_id := $2,
			p_user_id := $3
		)
	`

	var success bool
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterUserID, userID).Scan(&success)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}

	return nil
}

// CountUserOwnedTenants counts how many tenants a user owns
func (r *tenantRepository) CountUserOwnedTenants(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `SELECT tenancy.sp_count_user_owned_tenants($1)`

	var count int
	err := r.dbService.QueryRow(ctx, query, userID).Scan(&count)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return 0, pgErr
		}
		return 0, err
	}

	return count, nil
}

// ListTenantsWithCustomDB lists all tenants that have custom database URLs
func (r *tenantRepository) ListTenantsWithCustomDB(ctx context.Context) ([]*models.Tenant, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT id, slug, name, database_url, is_active, is_suspended, created_at
		 FROM tenancy.sp_list_tenants_with_custom_db()`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*models.Tenant
	for rows.Next() {
		var tenant models.Tenant
		err := rows.Scan(
			&tenant.ID,
			&tenant.Slug,
			&tenant.Name,
			&tenant.DatabaseURL,
			&tenant.IsActive,
			&tenant.IsSuspended,
			&tenant.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, &tenant)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

// ListTenantDirectory lists every active tenant with its database URL, nil for the shared platform
// database (INFRA-009).
func (r *tenantRepository) ListTenantDirectory(ctx context.Context) ([]*models.Tenant, error) {
	rows, err := r.dbService.Query(ctx, `SELECT id, slug, name, database_url FROM tenancy.sp_list_tenant_directory()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*models.Tenant
	for rows.Next() {
		var tenant models.Tenant
		if err := rows.Scan(&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.DatabaseURL); err != nil {
			return nil, err
		}
		tenants = append(tenants, &tenant)
	}
	return tenants, rows.Err()
}

// UpdateUserRole updates a user's role within a tenant
func (r *tenantRepository) UpdateUserRole(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) error {
	query := `
		SELECT * FROM tenancy.sp_update_user_role(
			p_tenant_id         := $1,
			p_requester_user_id := $2,
			p_user_id           := $3,
			p_role              := $4
		)
	`

	rows, err := r.dbService.Query(ctx, query, tenantID, requesterUserID, userID, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	rows.Close()

	return nil
}

// ListTenantAdminEmails returns the active owners and admins of one tenant — who an operational
// digest goes to (TRACK-016 D1).
func (r *tenantRepository) ListTenantAdminEmails(ctx context.Context, tenantID uuid.UUID) ([]*models.TenantAdminEmail, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tenancy.sp_list_tenant_admin_emails($1)`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipients := []*models.TenantAdminEmail{}
	for rows.Next() {
		var recipient models.TenantAdminEmail
		if err := rows.Scan(&recipient.UserID, &recipient.Email, &recipient.FirstName, &recipient.LastName); err != nil {
			return nil, err
		}
		recipients = append(recipients, &recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return recipients, nil
}
