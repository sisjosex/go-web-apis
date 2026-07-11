package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type UserAuditLog struct {
	ID                uuid.UUID        `json:"id"`
	Action            string           `json:"action"`
	TargetUserID      *uuid.UUID       `json:"target_user_id"`
	TargetUserEmail   *string          `json:"target_user_email"`
	PerformedBy       *uuid.UUID       `json:"performed_by"`
	PerformedByEmail  *string          `json:"performed_by_email"`
	Metadata          *json.RawMessage `json:"metadata"`
	Source            string           `json:"source"`
	CreatedAt         time.Time        `json:"created_at"`
}

type UserAuditListQuery struct {
	TenantID uuid.UUID
	Page     int
	Limit    int
	Search   string
	Action   string
	From     *string
	To       *string
}

type UserAuditListResponse struct {
	Entries    []UserAuditLog `json:"entries"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalPages int            `json:"total_pages"`
}
