package repositories

import (
	"context"
	"errors"

	"josex/web/modules/core/registry"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type moduleRepository struct {
	dbService coreServices.DatabaseService
}

// NewModuleRepository creates a new ModuleRepository.
func NewModuleRepository(dbService coreServices.DatabaseService) interfaces.ModuleRepository {
	return &moduleRepository{dbService: dbService}
}

// The module list is the platform's (tenancy.tenant_modules): it is read from the primary pool even
// inside a request routed to a dedicated tenant database, which does not hold it (TENANCY-003 D2).

func (r *moduleRepository) GetTenantModules(ctx context.Context, tenantID uuid.UUID) ([]models.TenantModule, error) {
	query := `SELECT * FROM tenancy.sp_get_tenant_modules($1)`
	rows, err := r.dbService.GetPrimaryPool().Query(ctx, query, tenantID)
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

// upsert runs sp_upsert_tenant_module on q — the pool, or a transaction.
func upsert(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	config := "{}"
	if dto.Config != nil && *dto.Config != "" {
		config = *dto.Config
	}

	row := q.QueryRow(ctx, `SELECT * FROM tenancy.sp_upsert_tenant_module($1, $2, $3, $4, $5)`,
		tenantID, moduleCode, dto.IsEnabled, config, enabledBy)

	var m models.TenantModule
	m.TenantID = tenantID
	if err := row.Scan(&m.ModuleCode, &m.IsEnabled, &m.Config, &m.EnabledAt, &m.EnabledBy); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return &m, nil
}

func (r *moduleRepository) UpsertTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	return upsert(ctx, r.dbService.GetPrimaryPool(), tenantID, moduleCode, dto, enabledBy)
}

// EnableWithRequirements turns on each of requirements (already off) and then moduleCode with dto, in
// one transaction: a module never ends up on without what it needs (TENANCY-003 D1).
func (r *moduleRepository) EnableWithRequirements(ctx context.Context, tenantID uuid.UUID, requirements []string, moduleCode string, dto models.UpsertTenantModuleDto, enabledBy uuid.UUID) (*models.TenantModule, error) {
	tx, err := r.dbService.GetPrimaryPool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, code := range requirements {
		if _, err := upsert(ctx, tx, tenantID, code, models.UpsertTenantModuleDto{IsEnabled: true}, enabledBy); err != nil {
			return nil, err
		}
	}
	module, err := upsert(ctx, tx, tenantID, moduleCode, dto, enabledBy)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return module, nil
}

func (r *moduleRepository) DisableTenantModule(ctx context.Context, tenantID uuid.UUID, moduleCode string) error {
	// The SP returns void, which pgx cannot scan: Exec, not QueryRow.
	query := `SELECT tenancy.sp_disable_tenant_module($1, $2)`
	_, err := r.dbService.GetPrimaryPool().Exec(ctx, query, tenantID, moduleCode)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	return nil
}

// ApplyVerticals runs on the primary pool, which holds tenancy.*: modules first, then the types, main
// module and capabilities, all or nothing (TENANCY-006 step 2).
func (r *moduleRepository) ApplyVerticals(ctx context.Context, tenantID uuid.UUID, w models.VerticalsWrite, enabledBy uuid.UUID) ([]string, *string, *string, error) {
	tx, err := r.dbService.GetPrimaryPool().Begin(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, code := range w.Missing {
		if _, err := upsert(ctx, tx, tenantID, code, models.UpsertTenantModuleDto{IsEnabled: true}, enabledBy); err != nil {
			return nil, nil, nil, err
		}
	}
	var (
		verticals []string
		primary   *string
		settings  *string
	)
	err = tx.QueryRow(ctx, `SELECT * FROM tenancy.sp_set_tenant_verticals($1, $2, $3, $4, $5, $6)`,
		tenantID, w.Verticals, w.PrimaryModule, w.Fallback, w.Capabilities, w.Restrict).Scan(&verticals, &primary, &settings)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, err
	}
	return verticals, primary, settings, nil
}

// SeedCategories resolves the request's database — the tenant's own when it has one — as every
// inventory write does.
func (r *moduleRepository) SeedCategories(ctx context.Context, tenantID uuid.UUID, names []string) (int, error) {
	var created int
	err := r.dbService.QueryRow(ctx, `SELECT inventory.sp_seed_categories($1, $2)`, tenantID, names).Scan(&created)
	return created, err
}

// TenantSetup reads tenancy.* on the primary pool, which holds it even for a dedicated-database tenant.
func (r *moduleRepository) TenantSetup(ctx context.Context, tenantID uuid.UUID) (business, team bool, verticals []string, err error) {
	err = r.dbService.GetPrimaryPool().QueryRow(ctx, `SELECT * FROM tenancy.sp_tenant_setup($1)`, tenantID).Scan(&business, &team, &verticals)
	return business, team, verticals, err
}

// CheckSetup hands the checker the service that resolves the request's database, as every module write does.
func (r *moduleRepository) CheckSetup(ctx context.Context, check registry.SetupChecker, tenantID uuid.UUID) (map[string]bool, error) {
	return check(ctx, r.dbService, tenantID)
}
