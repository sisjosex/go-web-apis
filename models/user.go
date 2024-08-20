package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primary_key" json:"id"`
	FirstName string     `gorm:"column:first_name" json:"first_name"`
	LastName  string     `gorm:"column:last_name" json:"last_name"`
	Email     string     `gorm:"column:email;unique" json:"email" conform:"trim,lowercase"`
	Password  string     `gorm:"column:password" json:"password"`
	Phone     string     `gorm:"column:phone" json:"phone"`
	Birthday  *time.Time `gorm:"column:birthday" json:"birthday"`
}
