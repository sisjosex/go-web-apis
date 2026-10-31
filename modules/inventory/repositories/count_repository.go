package repositories

import (
	"context"
	"encoding/json"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/inventory/models"

	"github.com/google/uuid"
)

// CountRepository calls the stock count SPs.
type CountRepository struct {
	dbService coreServices.DatabaseService
}

// NewCountRepository builds the repository.
func NewCountRepository(dbService coreServices.DatabaseService) *CountRepository {
	return &CountRepository{dbService: dbService}
}

// CreateCount starts a count and answers its id.
func (r *CountRepository) CreateCount(ctx context.Context, tenantID uuid.UUID, dto models.CreateCountDto, by *string) (string, error) {
	var id string
	err := r.dbService.QueryRow(ctx,
		`SELECT CAST(inventory.sp_create_count($1, $2::UUID, $3::UUID, $4, $5::UUID) AS VARCHAR)`,
		tenantID, dto.WarehouseID, dto.CategoryID, dto.Blind, by).Scan(&id)
	return id, err
}

// ListCounts answers a page with the chips' numbers.
func (r *CountRepository) ListCounts(ctx context.Context, tenantID uuid.UUID, query models.ListCountsQuery) (*models.ListCountsResponse, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT row_data, chips, total_count FROM inventory.sp_list_counts($1, $2, $3, $4)`,
		tenantID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &models.ListCountsResponse{Counts: []models.CountRow{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		var row, chips []byte
		if err := rows.Scan(&row, &chips, &out.TotalCount); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(chips, &out.Chips); err != nil {
			return nil, err
		}
		if row == nil {
			continue
		}
		var item models.CountRow
		if err := json.Unmarshal(row, &item); err != nil {
			return nil, err
		}
		out.Counts = append(out.Counts, item)
	}
	return out, rows.Err()
}

// GetCount answers a count with its lines.
func (r *CountRepository) GetCount(ctx context.Context, tenantID uuid.UUID, id string) (*models.CountDetail, error) {
	var body []byte
	if err := r.dbService.QueryRow(ctx, `SELECT inventory.sp_get_count($1, $2::UUID)`, tenantID, id).Scan(&body); err != nil {
		return nil, err
	}
	var out models.CountDetail
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetCountLines writes what was counted.
func (r *CountRepository) SetCountLines(ctx context.Context, tenantID uuid.UUID, id string, lines []models.CountLineDto) error {
	body, err := json.Marshal(lines)
	if err != nil {
		return err
	}
	_, err = r.dbService.Execute(ctx, `SELECT inventory.sp_set_count_lines($1, $2::UUID, $3::JSONB)`, tenantID, id, string(body))
	return err
}

// ScanCount counts one scan.
func (r *CountRepository) ScanCount(ctx context.Context, tenantID uuid.UUID, id, code string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_scan_count($1, $2::UUID, $3)`, tenantID, id, code)
	return err
}

// SetCountStatus pauses, resumes, sends to review or cancels.
func (r *CountRepository) SetCountStatus(ctx context.Context, tenantID uuid.UUID, id, action string) error {
	_, err := r.dbService.Execute(ctx, `SELECT inventory.sp_set_count_status($1, $2::UUID, $3)`, tenantID, id, action)
	return err
}

// CloseCount closes it into adjustments and answers the adjustment number.
func (r *CountRepository) CloseCount(ctx context.Context, tenantID uuid.UUID, id string, by *string) (string, error) {
	var number string
	err := r.dbService.QueryRow(ctx, `SELECT inventory.sp_close_count($1, $2::UUID, $3::UUID)`, tenantID, id, by).Scan(&number)
	return number, err
}
