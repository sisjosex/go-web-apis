package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	coreServices "josex/web/modules/core/services"
	userModels "josex/web/modules/users/models"
)

type userAuditRepository struct {
	dbService coreServices.DatabaseService
}

func NewUserAuditRepository(dbService coreServices.DatabaseService) *userAuditRepository {
	return &userAuditRepository{dbService: dbService}
}

func (r *userAuditRepository) Record(tenantID uuid.UUID, action string, targetUserID, performedBy *uuid.UUID, metadata *json.RawMessage, source string) error {
	var metadataArg any
	if metadata != nil && len(*metadata) > 0 {
		metadataArg = string(*metadata)
	}

	rows, err := r.dbService.Query(context.Background(), `
		SELECT users.sp_record_user_audit(
			p_tenant_id      := $1,
			p_action         := $2,
			p_target_user_id := $3,
			p_performed_by   := $4,
			p_metadata       := $5::JSONB,
			p_source         := $6
		)
	`, tenantID, action, targetUserID, performedBy, metadataArg, source)
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

func (r *userAuditRepository) List(query userModels.UserAuditListQuery) (*userModels.UserAuditListResponse, error) {
	var searchArg any
	if query.Search != "" {
		searchArg = query.Search
	}
	var actionArg any
	if query.Action != "" {
		actionArg = query.Action
	}

	rows, err := r.dbService.Query(context.Background(), `
		SELECT * FROM users.sp_list_user_audit(
			p_tenant_id := $1,
			p_page      := $2,
			p_limit     := $3,
			p_search    := $4,
			p_action    := $5,
			p_from      := $6::TIMESTAMPTZ,
			p_to        := $7::TIMESTAMPTZ
		)
	`, query.TenantID, query.Page, query.Limit, searchArg, actionArg, query.From, query.To)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	var entries []userModels.UserAuditLog
	var total int64
	for rows.Next() {
		var item userModels.UserAuditLog
		if err := rows.Scan(
			&item.ID,
			&item.Action,
			&item.TargetUserID,
			&item.TargetUserEmail,
			&item.PerformedBy,
			&item.PerformedByEmail,
			&item.Metadata,
			&item.Source,
			&item.CreatedAt,
			&total,
		); err != nil {
			return nil, err
		}
		entries = append(entries, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []userModels.UserAuditLog{}
	}

	totalPages := max(1, int(math.Ceil(float64(total)/float64(query.Limit))))

	return &userModels.UserAuditListResponse{
		Entries:    entries,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}, nil
}
