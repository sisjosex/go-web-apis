package user

import "time"

type CreateUserDto struct {
	FirstName string     `json:"first_name" binding:"required"`
	LastName  string     `json:"last_name" binding:"required"`
	Email     string     `form:"email" binding:"required,emailregex,emailexist" conform:"trim,lowercase"`
	Password  string     `json:"password" binding:"required"`
	Phone     string     `json:"phone"`
	Birthday  *time.Time `json:"birthday"`
}
