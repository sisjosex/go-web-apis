package controllers

import (
	"josex/web/common"
	"josex/web/interfaces"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionController struct {
	userService interfaces.UserService
}

func NewSessionController(userService interfaces.UserService) *SessionController {
	return &SessionController{
		userService: userService,
	}
}

// GetActiveSessions godoc
// @Summary Get active sessions
// @Description Get all sessions (active and inactive) for authenticated user
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {array} models.UserSession
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /auth/sessions [get]
// @Security ApiKeyAuth
func (sc *SessionController) GetActiveSessions(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	sessions, err := sc.userService.GetUserSessions(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// LogoutSession godoc
// @Summary Logout specific session
// @Description Logout a specific session by ID (must belong to authenticated user)
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Param id path string true "Session ID (UUID)"
// @Success 200 {object} map[string]bool
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /auth/sessions/{id} [delete]
// @Security ApiKeyAuth
func (sc *SessionController) LogoutSession(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	err = sc.userService.LogoutSession(userID, sessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// LogoutAllSessions godoc
// @Summary Logout all sessions
// @Description Logout all sessions for authenticated user except current one
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} map[string]int
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /auth/sessions [delete]
// @Security ApiKeyAuth
func (sc *SessionController) LogoutAllSessions(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildErrorSingle(common.InvalidUUID))
		return
	}

	// Get current session ID to exclude it from logout
	currentSessionIDStr := c.GetString("session_id")
	var currentSessionID *uuid.UUID
	if currentSessionIDStr != "" {
		parsedID, err := uuid.Parse(currentSessionIDStr)
		if err == nil {
			currentSessionID = &parsedID
		}
	}

	count, err := sc.userService.LogoutAllSessions(userID, currentSessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.BuildError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"logged_out_count": count})
}
