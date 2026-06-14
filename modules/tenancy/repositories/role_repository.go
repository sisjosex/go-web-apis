package repositories

import (
	"context"
	"errors"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type roleRepository struct {
	dbService coreServices.DatabaseService
}

// NewRoleRepository creates a new RoleRepository.
func NewRoleRepository(dbService coreServices.DatabaseService) interfaces.RoleRepository {
	return &roleRepository{dbService: dbService}
}

func (r *roleRepository) CreateRole(ctx context.Context, tenantID, requesterID uuid.UUID, dto *models.CreateRoleDto) (*models.Role, error) {
	query := `
		SELECT * FROM tenancy.sp_create_role(
			p_tenant_id         := $1,
			p_requester_user_id := $2,
			p_name              := $3,
			p_description       := $4
		)
	`
	role := &models.Role{}
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, dto.Name, dto.Description).Scan(
		&role.ID,
		&role.TenantID,
		&role.Name,
		&role.Description,
		&role.IsSystem,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return role, nil
}

func (r *roleRepository) UpdateRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, dto *models.UpdateRoleDto) (*models.Role, error) {
	query := `
		SELECT * FROM tenancy.sp_update_role(
			p_tenant_id         := $1,
			p_requester_user_id := $2,
			p_role_id           := $3,
			p_name              := $4,
			p_description       := $5
		)
	`
	var name *string
	if dto.Name != nil {
		name = dto.Name
	}

	role := &models.Role{}
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, roleID, name, dto.Description).Scan(
		&role.ID,
		&role.TenantID,
		&role.Name,
		&role.Description,
		&role.IsSystem,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return role, nil
}

func (r *roleRepository) DeleteRole(ctx context.Context, tenantID, requesterID, roleID uuid.UUID) error {
	query := `SELECT tenancy.sp_delete_role($1, $2, $3)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, roleID).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *roleRepository) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]*models.RoleListItem, error) {
	query := `SELECT * FROM tenancy.sp_list_roles($1)`

	rows, err := r.dbService.Query(ctx, query, tenantID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	items := make([]*models.RoleListItem, 0)
	for rows.Next() {
		item := &models.RoleListItem{}
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.Name,
			&item.Description,
			&item.IsSystem,
			&item.PermissionCount,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *roleRepository) GetRole(ctx context.Context, tenantID, roleID uuid.UUID) (*models.RoleDetail, error) {
	query := `SELECT * FROM tenancy.sp_get_role($1, $2)`

	role := &models.RoleDetail{}
	err := r.dbService.QueryRow(ctx, query, tenantID, roleID).Scan(
		&role.ID,
		&role.TenantID,
		&role.Name,
		&role.Description,
		&role.IsSystem,
		&role.Permissions,
		&role.CreatedAt,
		&role.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	if role.Permissions == nil {
		role.Permissions = []string{}
	}

	return role, nil
}

func (r *roleRepository) SetRolePermissions(ctx context.Context, tenantID, requesterID, roleID uuid.UUID, permCodes []string) error {
	query := `SELECT tenancy.sp_set_role_permissions($1, $2, $3, $4)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, roleID, permCodes).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *roleRepository) AssignUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error {
	query := `SELECT tenancy.sp_assign_user_role($1, $2, $3, $4)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, userID, roleID).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *roleRepository) RevokeUserRole(ctx context.Context, tenantID, requesterID, userID, roleID uuid.UUID) error {
	query := `SELECT tenancy.sp_revoke_user_role($1, $2, $3, $4)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, tenantID, requesterID, userID, roleID).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *roleRepository) ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserRole, error) {
	query := `SELECT * FROM tenancy.sp_list_user_roles($1, $2)`

	rows, err := r.dbService.Query(ctx, query, userID, tenantID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	items := make([]*models.UserRole, 0)
	for rows.Next() {
		item := &models.UserRole{}
		if err := rows.Scan(
			&item.RoleID,
			&item.RoleName,
			&item.Description,
			&item.IsSystem,
			&item.AssignedBy,
			&item.AssignedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
