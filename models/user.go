package models

import "josex/web/utils"

type User struct {
	ID                string          `json:"id" binding:"required"`
	FirstName         string          `json:"first_name" binding:"required"`
	LastName          string          `json:"last_name" binding:"required"`
	Email             string          `form:"email" binding:"required,email-valid" conform:"trim,lowercase"`
	Password          string          `json:"password" binding:"required"`
	Phone             string          `json:"phone"`
	Birthday          *utils.DateOnly `json:"birthday"`
	ProfilePictureUrl string          `json:"profile_picture_url"`
	Bio               string          `json:"bio"`
	WebsiteUrl        string          `json:"website_url"`
}
