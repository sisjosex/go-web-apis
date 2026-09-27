// Package push is the Pusher adapter (TRACK-012 D2): FCM HTTP v1 over a plain HTTP call, one request
// per message — the same request firebase-admin-go makes, without its dependency tree.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	trackingConfig "josex/web/modules/tracking/config"
	"josex/web/modules/tracking/interfaces"

	"github.com/sony/gobreaker/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	// Endpoint is FCM's host; tests point an adapter at a fake one.
	Endpoint = "https://fcm.googleapis.com"
	scope    = "https://www.googleapis.com/auth/firebase.messaging"
	timeout  = 5 * time.Second
	// Breaker policy, as the Valhalla adapter's: five outages in a row open it for 30 s.
	breakerFailures = 5
	breakerOpenFor  = 30 * time.Second
)

// New builds the FCM adapter from config, or answers nil when FCM_PROJECT_ID or
// FCM_CREDENTIALS_FILE is unset: without credentials a notice stays in the feed only. ctx is the
// process's: the token source fetches access tokens with it for as long as the adapter lives.
func New(ctx context.Context, cfg *trackingConfig.TrackingConfig) (interfaces.Pusher, error) {
	if cfg.FCMProjectID == "" || cfg.FCMCredentialsFile == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(cfg.FCMCredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("fcm credentials: %w", err)
	}
	// The three fields of a service-account key a signed-JWT grant needs.
	var key struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(raw, &key); err != nil || key.ClientEmail == "" || key.PrivateKey == "" {
		return nil, fmt.Errorf("fcm credentials: not a service-account key")
	}
	grant := &jwt.Config{Email: key.ClientEmail, PrivateKey: []byte(key.PrivateKey), Scopes: []string{scope}, TokenURL: key.TokenURI}
	if grant.TokenURL == "" {
		grant.TokenURL = "https://oauth2.googleapis.com/token"
	}
	// The grant's source caches the access token until it nears expiry: one token call an hour.
	return NewFCM(cfg.FCMProjectID, grant.TokenSource(ctx), Endpoint), nil
}

// FCM sends through FCM HTTP v1.
type FCM struct {
	url     string
	tokens  oauth2.TokenSource
	client  *http.Client
	breaker *gobreaker.CircuitBreaker[any]
}

// NewFCM is the adapter for one Firebase project; endpoint is Endpoint outside tests.
func NewFCM(projectID string, tokens oauth2.TokenSource, endpoint string) *FCM {
	return &FCM{
		url:    endpoint + "/v1/projects/" + projectID + "/messages:send",
		tokens: tokens,
		client: &http.Client{Timeout: timeout},
		breaker: gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
			Name:        "fcm",
			Timeout:     breakerOpenFor,
			ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= breakerFailures },
			// A dead token or a refused message is FCM working; only outages count toward opening.
			IsSuccessful: func(err error) bool {
				return err == nil || errors.Is(err, interfaces.ErrPushUnregistered) || errors.Is(err, interfaces.ErrPushRejected)
			},
		}),
	}
}

// Send delivers one message. The phone looks up notice_<type> (title) and notice_<type>_body (body)
// in its own strings and fills the body with Args.
func (f *FCM) Send(ctx context.Context, msg interfaces.PushMessage) error {
	_, err := f.breaker.Execute(func() (any, error) { return nil, f.send(ctx, msg) })
	if err == nil || errors.Is(err, interfaces.ErrPushUnregistered) || errors.Is(err, interfaces.ErrPushRejected) || errors.Is(err, interfaces.ErrPushUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %w", interfaces.ErrPushUnavailable, err)
}

type fcmRequest struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token   string            `json:"token"`
	Data    map[string]string `json:"data,omitempty"`
	Android fcmAndroid        `json:"android"`
	APNS    fcmAPNS           `json:"apns"`
}

type fcmAndroid struct {
	CollapseKey  string `json:"collapse_key,omitempty"`
	Notification struct {
		TitleLocKey string   `json:"title_loc_key"`
		BodyLocKey  string   `json:"body_loc_key"`
		BodyLocArgs []string `json:"body_loc_args,omitempty"`
		Tag         string   `json:"tag,omitempty"`
	} `json:"notification"`
}

type fcmAPNS struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload struct {
		Aps struct {
			Alert struct {
				TitleLocKey string   `json:"title-loc-key"`
				LocKey      string   `json:"loc-key"`
				LocArgs     []string `json:"loc-args,omitempty"`
			} `json:"alert"`
		} `json:"aps"`
	} `json:"payload"`
}

type fcmError struct {
	Error struct {
		Status  string `json:"status"`
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

func (f *FCM) send(ctx context.Context, msg interfaces.PushMessage) error {
	token, err := f.tokens.Token()
	if err != nil {
		return fmt.Errorf("%w: access token: %w", interfaces.ErrPushUnavailable, err)
	}
	body, err := json.Marshal(fcmRequest{Message: toFCM(msg)})
	if err != nil {
		return fmt.Errorf("%w: %w", interfaces.ErrPushRejected, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	token.SetAuthHeader(req)
	res, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return classify(res.StatusCode, payload)
}

// classify maps FCM's answer to the port's errors: UNREGISTERED is a dead token, 429 and 5xx an
// outage worth retrying, any other 4xx a refusal a retry does not change.
func classify(status int, payload []byte) error {
	if status < 300 {
		return nil
	}
	var fe fcmError
	_ = json.Unmarshal(payload, &fe)
	for _, d := range fe.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return interfaces.ErrPushUnregistered
		}
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		return fmt.Errorf("%w: fcm %d %s", interfaces.ErrPushUnavailable, status, fe.Error.Status)
	}
	return fmt.Errorf("%w: fcm %d %s", interfaces.ErrPushRejected, status, fe.Error.Status)
}

func toFCM(msg interfaces.PushMessage) fcmMessage {
	title, body := "notice_"+msg.Type, "notice_"+msg.Type+"_body"
	m := fcmMessage{Token: msg.Token, Data: msg.Data}
	m.Android.CollapseKey = msg.CollapseID
	m.Android.Notification.TitleLocKey = title
	m.Android.Notification.BodyLocKey = body
	m.Android.Notification.BodyLocArgs = msg.Args
	m.Android.Notification.Tag = msg.CollapseID
	if msg.CollapseID != "" {
		m.APNS.Headers = map[string]string{"apns-collapse-id": msg.CollapseID}
	}
	m.APNS.Payload.Aps.Alert.TitleLocKey = title
	m.APNS.Payload.Aps.Alert.LocKey = body
	m.APNS.Payload.Aps.Alert.LocArgs = msg.Args
	return m
}
