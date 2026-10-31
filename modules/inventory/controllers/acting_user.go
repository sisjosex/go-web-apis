package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// actingUser is the signed-in user, for the SPs that record who changed a row (INV-020 D1).
func actingUser(c *gin.Context) *string {
	userID := c.GetString("user_id")
	if userID == "" {
		return nil
	}
	return &userID
}

// actingUserID is actingUser for the batch layer, which speaks uuid.
func actingUserID(c *gin.Context) *uuid.UUID {
	parsed, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil
	}
	return &parsed
}
