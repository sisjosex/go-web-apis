package models

type SessionUser struct {
	UserId    string `json:"user_id" binding:"required"`
	SessionId string `json:"session_id" binding:"required"`
	IsActive  *bool  `json:"is_active"`
}
