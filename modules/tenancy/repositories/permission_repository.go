package repositories

import (
	"context"
	"errors"
	"time"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type permissionRepository struct {
	dbService coreServices.DatabaseService
}

// NewPermissionRepository creates a new PermissionRepository
func NewPermissionRepository(dbService coreServices.DatabaseService) interfaces.PermissionRepository {
	return &permissionRepository{dbService: dbService}
}

func (r *permissionRepository) AssignPermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error {
	query := `SELECT tenancy.sp_assign_user_permission($1, $2, $3, $4)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, requesterID, tenantID, targetUserID, permCode).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *permissionRepository) RevokePermission(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID, permCode string) error {
	query := `SELECT tenancy.sp_revoke_user_permission($1, $2, $3, $4)`
	var ok bool
	err := r.dbService.QueryRow(ctx, query, requesterID, tenantID, targetUserID, permCode).Scan(&ok)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

func (r *permissionRepository) ListUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]*models.UserPermissionResponse, error) {
	query := `SELECT * FROM tenancy.sp_list_user_permissions($1, $2)`

	rows, err := r.dbService.Query(ctx, query, userID, tenantID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	perms := make([]*models.UserPermissionResponse, 0)
	for rows.Next() {
		var code string
		var grantedBy uuid.UUID
		var grantedAt time.Time
		if err := rows.Scan(&code, &grantedBy, &grantedAt); err != nil {
			return nil, err
		}
		perms = append(perms, &models.UserPermissionResponse{
			Code:      code,
			GrantedBy: grantedBy,
			GrantedAt: grantedAt,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return perms, nil
}
