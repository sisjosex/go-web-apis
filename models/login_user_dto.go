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

type LoginExternalDto struct {
	AuthProviderName string    `json:"auth_provider_name" conform:"trim,lowercase"`
	AuthProviderId   string    `json:"auth_provider_id" binding:"required" conform:"trim,lowercase"`
	DeviceId         uuid.UUID `json:"device_id" binding:"uuidv4"`
	FirstName        string    `json:"first_name"`
	LastName         string    `json:"last_name"`
	Email            string    `form:"email" binding:"omitempty,email-valid" conform:"trim,lowercase"`
	Phone            string    `json:"phone"`
	Birthday         *DateOnly `json:"birthday" time_format:"2006-01-02"`
	IpAddress        string    `json:"ip_address"`
	DeviceInfo       string    `json:"device_info"`
	DeviceOs         string    `json:"device_os"`
	Browser          string    `json:"browser"`
	UserAgent        string    `json:"user_agent"`
}
