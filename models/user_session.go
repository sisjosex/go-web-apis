package models

import (
	"github.com/google/uuid"
)

type UserSession struct {
	SessionID  uuid.UUID  `json:"session_id"`
	UserID     uuid.UUID  `json:"user_id"`
	DeviceID   *uuid.UUID `json:"device_id,omitempty"`
	DeviceOS   string     `json:"device_os"`
	Browser    string     `json:"browser"`
	IpAddress  string     `json:"ip_address"`
	LoginTime  DateTime   `json:"login_time"`
	LastActive DateTime   `json:"last_active"`
	LogoutTime *DateTime  `json:"logout_time,omitempty"`
	IsActive   bool       `json:"is_active"`
}
