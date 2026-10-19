package repositories

import (
	"context"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// ListRiderAccessLog answers one page of who outside the operator read the rider (TRACK-063).
func (r *TrackingRepository) ListRiderAccessLog(ctx context.Context, tenantID, riderID uuid.UUID, query models.ListRiderAccessLogQuery) (*models.ListRiderAccessLogResponse, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_rider_access_log(
			p_tenant_id := $1,
			p_rider_id  := $2,
			p_from      := $3::DATE,
			p_to        := $4::DATE,
			p_page      := $5,
			p_page_size := $6
		)
	`, tenantID, riderID, query.From, query.To, query.Page, query.PageSize)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.RiderAccessLogFailed)
	}
	defer rows.Close()
	result := &models.ListRiderAccessLogResponse{Entries: []models.RiderAccessEntry{}, Page: query.Page, PageSize: query.PageSize}
	for rows.Next() {
		var e models.RiderAccessEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.Email, &e.FirstName, &e.LastName, &e.Level, &e.Action,
			&e.Reads, &e.FirstAt, &e.LastAt, &result.TotalCount); err != nil {
			return nil, scopedErr(err, trackingErrors.RiderAccessLogFailed)
		}
		result.Entries = append(result.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, scopedErr(err, trackingErrors.RiderAccessLogFailed)
	}
	return result, nil
}

// AnonymiseRiderContacts clears the contact data of the tenant's riders inactive for more than
// retentionDays and answers the accounts it unlinked (TRACK-063 D3).
func (r *TrackingRepository) AnonymiseRiderContacts(ctx context.Context, tenantID uuid.UUID, retentionDays int) ([]uuid.UUID, error) {
	rows, err := r.dbService.Query(ctx,
		`SELECT DISTINCT user_id FROM tracking.sp_anonymise_rider_contacts(p_tenant_id := $1, p_retention_days := $2)`,
		tenantID, retentionDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		users = append(users, id)
	}
	return users, rows.Err()
}

// PruneRiderAccessLog drops the tenant's access log rows last read more than days ago.
func (r *TrackingRepository) PruneRiderAccessLog(ctx context.Context, tenantID uuid.UUID, days int) (int, error) {
	var deleted int
	err := r.dbService.QueryRow(ctx,
		`SELECT tracking.sp_prune_rider_access_log(p_tenant_id := $1, p_days := $2)`, tenantID, days).Scan(&deleted)
	return deleted, err
}
