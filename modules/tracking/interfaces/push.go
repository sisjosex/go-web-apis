package interfaces

import (
	"context"
	"errors"
)

// PushMessage is one notice on its way to one phone (TRACK-012). The phone translates it: Type picks
// its strings and Args fill them, so the server never renders a language. Data rides along for the
// tap; CollapseID makes a later notice replace the one already shown instead of stacking a second
// one; Channel is the Android channel the phone created for it (MOBILE-004). Silent is a data-only
// message the app handles without showing anything — Type, Args and Channel are then unused, and a
// newer one with the same CollapseID replaces one still waiting for the phone (TRACK-030 D3).
type PushMessage struct {
	Token      string
	Type       string
	Args       []string
	Data       map[string]string
	CollapseID string
	Channel    string
	Silent     bool
	// Urgent sends a Silent (data-only) message at high priority: the phone draws it itself, now —
	// the trip-progress notification (MOBILE-024), which a dozing phone must not hold back.
	Urgent bool
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
