package models

type SessionToken struct {
	AccessToken string `json:"access_token" binding:"required"`
}
