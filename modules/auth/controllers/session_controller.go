package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	coreErrors "josex/web/modules/core/errors"
)

// Ensure authModels is used (for Swagger annotations)
var _ authModels.UserSession

type SessionController struct {
	authService authInterfaces.AuthService
}

func NewSessionController(authService authInterfaces.AuthService) *SessionController {
	return &SessionController{
		authService: authService,
	}
}

// GetActiveSessions godoc
// @Summary Get active sessions
// @Description Get all sessions (active and inactive) for authenticated user
// @Tags Sessions
// @Accept  json
// @Produce  json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {array} authModels.UserSession
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /auth/sessions [get]
// @Security ApiKeyAuth
func (sc *SessionController) GetActiveSessions(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	sessions, err := sc.authService.GetUserSessions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	// None is `[]`, never `null`; the session this request came with says so.
	if sessions == nil {
		sessions = []authModels.UserSession{}
	}
	current := c.GetString("session_id")
	for i := range sessions {
		sessions[i].IsCurrent = sessions[i].SessionID.String() == current
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
// @Failure 400 {object} coreErrors.ErrorResponse "Already signed out"
// @Failure 401 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse "No such session for this user"
// @Router /auth/sessions/{id} [delete]
// @Security ApiKeyAuth
func (sc *SessionController) LogoutSession(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	err = sc.authService.LogoutSession(userID, sessionID)
	if err != nil {
		c.JSON(logoutSessionStatus(err), coreErrors.BuildError(c, err))
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
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /auth/sessions [delete]
// @Security ApiKeyAuth
func (sc *SessionController) LogoutAllSessions(c *gin.Context) {
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
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

	count, err := sc.authService.LogoutAllSessions(userID, currentSessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"logged_out_count": count})
}

// logoutSessionStatus maps the SP's session errors: another user's session is as absent as a missing
// one (404), a session already signed out is the caller's mistake (400), anything else is ours.
func logoutSessionStatus(err error) int {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return http.StatusInternalServerError
	}
	switch pgErr.Message {
	case authErrors.SessionNotFound, authErrors.SessionUnauthorized:
		return http.StatusNotFound
	case authErrors.SessionAlreadyLoggedOut:
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
