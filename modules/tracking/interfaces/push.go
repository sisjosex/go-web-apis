package interfaces

import (
	"context"
	"errors"
)

// PushMessage is one notice on its way to one phone (TRACK-012). The phone translates it: Type picks
// its strings and Args fill them, so the server never renders a language. Data rides along for the
// tap; CollapseID makes a resend replace the notice already shown instead of stacking a second one.
type PushMessage struct {
	Token      string
	Type       string
	Args       []string
	Data       map[string]string
	CollapseID string
}

// Pusher is the push port (TRACK-012 D2): FCM today, for Android and iOS alike.
type Pusher interface {
	// Send delivers one message, or answers ErrPushUnregistered / ErrPushRejected /
	// ErrPushUnavailable.
	Send(ctx context.Context, msg PushMessage) error
}

var (
	// ErrPushUnregistered is the token gone for good — the app was uninstalled or the token rotated.
	ErrPushUnregistered = errors.New("push: token unregistered")
	// ErrPushRejected is a message the service refused for a reason a retry does not change.
	ErrPushRejected = errors.New("push: message rejected")
	// ErrPushUnavailable is the service down, slow, throttling or behind an open breaker: retry later.
	ErrPushUnavailable = errors.New("push: unavailable")
)
