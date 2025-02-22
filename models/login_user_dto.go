package models

import "github.com/google/uuid"

type LoginUserDto struct {
	Email      string    `json:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password   string    `json:"password" binding:"required"`
	DeviceId   uuid.UUID `json:"device_id" binding:"required,uuidv4"`
	IpAddress  string    `json:"ip_address"`
	DeviceInfo string    `json:"device_info"`
	DeviceOs   string    `json:"device_os"`
	Browser    string    `json:"browser"`
	UserAgent  string    `json:"user_agent"`
}
