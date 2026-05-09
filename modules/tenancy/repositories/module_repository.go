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

type moduleRepository struct {
	dbService coreServices.DatabaseService
}

// NewModuleRepository creates a new ModuleRepository.
func NewModuleRepository(dbService coreServices.DatabaseService) interfaces.ModuleRepository {
	return &moduleRepository{dbService: dbService}
}

func (r *moduleRepository) GetTenantModules(ctx context.Context, tenantID uuid.UUID) ([]models.TenantModule, error) {
	query := `SELECT * FROM tenancy.sp_get_tenant_modules($1)`
	rows, err := r.dbService.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TenantModule
	for rows.Next() {
		var m models.TenantModule
		m.TenantID = tenantID
		if err := rows.Scan(&m.ModuleCode, &m.IsEnabled, &m.Config, &m.EnabledAt, &m.EnabledBy); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *moduleRepository) UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	query := `SELECT * FROM tenancy.sp_upsert_tenant_module($1, $2, $3, $4, $5)`

	config := "{}"
	if dto.Config != nil && *dto.Config != "" {
		config = *dto.Config
	}

	row := r.dbService.QueryRow(ctx, query, tenantID, moduleCode, dto.IsEnabled, config, enabledBy)

	var m models.TenantModule
	m.TenantID = tenantID
	err := row.Scan(&m.ModuleCode, &m.IsEnabled, &m.Config, &m.EnabledAt, &m.EnabledBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return &m, nil
}

func (r *moduleRepository) DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error {
	query := `SELECT tenancy.sp_disable_tenant_module($1, $2)`
	row := r.dbService.QueryRow(ctx, query, tenantID, moduleCode)
	var dummy any
	err := row.Scan(&dummy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}
