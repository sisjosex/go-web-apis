package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	coreServices "josex/web/modules/core/services"
	importModels "josex/web/modules/import/models"
)

// ImportRepository keeps what the engine needs between files (IMPORT-002): the
// mappings remembered per file shape (D3) and the history of runs. Every call
// goes through the request's database, the tenant's own when it has one.
type ImportRepository struct {
	db coreServices.DatabaseService
}

func NewImportRepository(db coreServices.DatabaseService) *ImportRepository {
	return &ImportRepository{db: db}
}

// GetMapping answers nil, nil when the file shape has no mapping yet.
func (r *ImportRepository) GetMapping(ctx context.Context, tenantID uuid.UUID, resource, signature string) (map[string]string, error) {
	var raw []byte
	err := r.db.QueryRow(ctx,
		`SELECT import.sp_get_column_mapping(p_tenant_id := $1, p_resource := $2, p_header_signature := $3)`,
		tenantID, resource, signature).Scan(&raw)
	if err != nil || raw == nil {
		return nil, err
	}
	var mapping map[string]string
	if err := json.Unmarshal(raw, &mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func (r *ImportRepository) UpsertMapping(ctx context.Context, tenantID uuid.UUID, resource, signature string, mapping map[string]string) error {
	raw, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	_, err = r.db.Execute(ctx,
		`SELECT import.sp_upsert_column_mapping(p_tenant_id := $1, p_resource := $2, p_header_signature := $3, p_mapping := $4)`,
		tenantID, resource, signature, raw)
	return err
}

func (r *ImportRepository) RecordRun(ctx context.Context, tenantID uuid.UUID, meta importModels.RunMeta) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT import.sp_record_run(p_tenant_id := $1, p_resource := $2, p_file_name := $3, p_total := $4,
		        p_created := $5, p_updated := $6, p_skipped := $7, p_failed := $8, p_warnings := $9, p_performed_by := $10)`,
		tenantID, meta.Resource, meta.FileName, meta.Total, meta.Created, meta.Updated, meta.Skipped, meta.Failed,
		meta.Warnings, meta.PerformedBy).Scan(&id)
	return id, err
}

func (r *ImportRepository) ListRuns(ctx context.Context, tenantID uuid.UUID, query importModels.RunsQuery) (*importModels.RunsResponse, error) {
	var resource *string
	if query.Resource != "" {
		resource = &query.Resource
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, resource, file_name, total, created, updated, skipped, failed, performed_by_name, created_at, total_count
		 FROM import.sp_list_runs(p_tenant_id := $1, p_resource := $2, p_page := $3, p_page_size := $4)`,
		tenantID, resource, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &importModels.RunsResponse{Runs: []importModels.RunRow{}}
	for rows.Next() {
		var run importModels.RunRow
		if err := rows.Scan(&run.ID, &run.Resource, &run.FileName, &run.Total, &run.Created, &run.Updated,
			&run.Skipped, &run.Failed, &run.PerformedByName, &run.CreatedAt, &out.TotalCount); err != nil {
			return nil, err
		}
		out.Runs = append(out.Runs, run)
	}
	return out, rows.Err()
}

func (r *ImportRepository) LastRuns(ctx context.Context, tenantID uuid.UUID) (map[string]time.Time, error) {
	rows, err := r.db.Query(ctx, `SELECT resource, last_run_at FROM import.sp_last_runs(p_tenant_id := $1)`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]time.Time{}
	for rows.Next() {
		var resource string
		var at time.Time
		if err := rows.Scan(&resource, &at); err != nil {
			return nil, err
		}
		out[resource] = at
	}
	return out, rows.Err()
}
