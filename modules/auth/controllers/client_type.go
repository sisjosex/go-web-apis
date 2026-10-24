package controllers

import (
	"net/http"
	"strings"

	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"

	"github.com/gin-gonic/gin"
)

// Client types a session may be opened from. The set matches the CHECK on
// auth.user_sessions.client_type — the API refuses anything else before the SP sees it.
const (
	ClientTypeWeb    = "web"
	ClientTypeMobile = "mobile"
)

// clientType reads the client a session is being opened from (TRACK-015 D2). Every login path goes
// through here so the header is read, defaulted and refused in exactly one place: an absent header
// is the web app, and an unknown value is a 400 rather than a silently mislabelled session.
// It answers the request itself when it refuses, and reports whether the caller may continue.
func clientType(c *gin.Context) (string, error) {
	declared := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Client-Type")))
	switch declared {
	case "":
		return ClientTypeWeb, nil
	case ClientTypeWeb, ClientTypeMobile:
		return declared, nil
	default:
		return "", api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, authErrors.ClientTypeInvalid))
	}
}
