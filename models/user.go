package models

import "time"

type User struct {
	ID        int        `gorm:"primaryKey:autoIncrement" json:"id"`
	FirstName string     `gorm:"column:first_name" json:"first_name"`
	LastName  string     `gorm:"column:last_name" json:"last_name"`
	Email     string     `gorm:"column:email;unique" json:"email" conform:"trim,lowercase"`
	Password  string     `gorm:"column:password" json:"password"`
	Phone     string     `gorm:"column:phone" json:"phone"`
	Birthday  *time.Time `gorm:"column:birthday" json:"birthday"`
}
