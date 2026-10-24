package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	authErrors "josex/web/modules/auth/errors"
	authInterfaces "josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/core/api"
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

// GetActiveSessions is GET /sessions: Get active sessions.
func (sc *SessionController) GetActiveSessions(ctx context.Context, _ *api.None) (*api.Out[[]authModels.UserSession], error) {
	c := api.Gin(ctx)
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	sessions, err := sc.authService.GetUserSessions(userID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	// None is `[]`, never `null`; the session this request came with says so.
	if sessions == nil {
		sessions = []authModels.UserSession{}
	}
	current := c.GetString("session_id")
	for i := range sessions {
		sessions[i].IsCurrent = sessions[i].SessionID.String() == current
	}

	return api.OK(sessions)
}

// LogoutSessionInput is LogoutSession's request.
type LogoutSessionInput struct {
	ID string `path:"id"`
}

// LogoutSessionResponse is LogoutSession's answer.
type LogoutSessionResponse struct {
	Success bool `json:"success"`
}

// LogoutSession is DELETE /sessions/{id}: Logout specific session.
func (sc *SessionController) LogoutSession(ctx context.Context, in *LogoutSessionInput) (*api.Out[LogoutSessionResponse], error) {
	c := api.Gin(ctx)
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	sessionIDStr := in.ID
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	err = sc.authService.LogoutSession(userID, sessionID)
	if err != nil {
		return nil, api.Fail(logoutSessionStatus(err), coreErrors.BuildError(c, err))
	}

	return api.OK(LogoutSessionResponse{Success: true})
}

// LogoutAllSessionsResponse is LogoutAllSessions's answer.
type LogoutAllSessionsResponse struct {
	LoggedOutCount int `json:"logged_out_count"`
}

// LogoutAllSessions is DELETE /sessions: Logout all sessions.
func (sc *SessionController) LogoutAllSessions(ctx context.Context, _ *api.None) (*api.Out[LogoutAllSessionsResponse], error) {
	c := api.Gin(ctx)
	userIDStr := c.GetString("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
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
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(LogoutAllSessionsResponse{LoggedOutCount: count})
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
