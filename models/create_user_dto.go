package models

import (
	"josex/web/utils"
)

type CreateUserDto struct {
	FirstName         string         `json:"first_name" binding:"required"`
	LastName          string         `json:"last_name" binding:"required"`
	Email             string         `form:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password          string         `json:"password" binding:"required"`
	Phone             string         `json:"phone"`
	Birthday          utils.DateOnly `json:"birthday" time_format:"2006-01-02"`
	ProfilePictureUrl string         `json:"profile_picture_url"`
	Bio               string         `json:"bio"`
	WebsiteUrl        string         `json:"website_url"`
}
