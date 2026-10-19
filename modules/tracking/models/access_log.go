package models

import (
	"time"

	"github.com/google/uuid"
)

// Who outside the operator read a rider (TRACK-063): one entry per reader, action and hour.

// RiderAccessEntry is one hour of one reader's reads of the rider. Level is portal (a guardian) or
// organization (the client's user); Action is status, today, live or absences. The names are nil when
// the reader's account is gone.
type RiderAccessEntry struct {
	ID        int64     `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Email     *string   `json:"email"`
	FirstName *string   `json:"first_name"`
	LastName  *string   `json:"last_name"`
	Level     string    `json:"level"`
	Action    string    `json:"action"`
	Reads     int32     `json:"reads"`
	FirstAt   time.Time `json:"first_at"`
	LastAt    time.Time `json:"last_at"`
}

// ListRiderAccessLogQuery binds GET /tracking/riders/:id/audit: the hours touching [from, to], either
// bound open when empty.
type ListRiderAccessLogQuery struct {
	From     *string `form:"from" binding:"omitempty,datetime=2006-01-02"`
	To       *string `form:"to" binding:"omitempty,datetime=2006-01-02"`
	Page     int     `form:"page,default=1" binding:"min=1"`
	PageSize int     `form:"page_size,default=20" binding:"min=1,max=100"`
}

type ListRiderAccessLogResponse struct {
	Entries    []RiderAccessEntry `json:"entries"`
	TotalCount int64              `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}
