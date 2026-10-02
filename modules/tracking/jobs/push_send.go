package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// TaskPushSend is one feed row pushed to its recipient's phones (TRACK-012). Its TaskID is the row's
// id, so an event replayed before the task ran enqueues nothing new.
const TaskPushSend = "tracking:push-send"

// pushSendRetries bounds how long a notice keeps trying: asynq's backoff puts the fifth retry some
// ten minutes out, past which "the bus is near" is no longer news. The feed row stays either way.
const pushSendRetries = 5

// PushSend is the task's payload: everything the message needs, so sending reads nothing of the
// tenant — only the recipient's tokens, from the main database.
type PushSend struct {
	NotificationID string            `json:"notification_id"`
	UserID         string            `json:"user_id"`
	Type           string            `json:"type"`
	Args           []string          `json:"args"`
	Data           map[string]string `json:"data"`
	// CollapseID and Channel are empty on a task enqueued before MOBILE-004: the notice id collapses
	// it and the phone's default channel shows it.
	CollapseID string `json:"collapse_id,omitempty"`
	Channel    string `json:"channel,omitempty"`
}

// PushSendHandler sends one notice to each of the recipient's phones. A token FCM answers UNREGISTERED
// is deleted; a refused message is logged and dropped; an outage fails the task so asynq retries it
// with backoff — a phone that already got it sees the resend replace it (CollapseID).
func PushSendHandler(db coreServices.DatabaseService, pusher interfaces.Pusher) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload PushSend
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("push-send payload: %v: %w", err, asynq.SkipRetry)
		}
		userID, err := uuid.Parse(payload.UserID)
		if err != nil {
			return fmt.Errorf("push-send user_id %q: %v: %w", payload.UserID, err, asynq.SkipRetry)
		}
		collapseID := payload.CollapseID
		if collapseID == "" {
			collapseID = payload.NotificationID
		}
		return pushToUser(ctx, db, pusher, userID, interfaces.PushMessage{
			Type: payload.Type, Args: payload.Args, Data: payload.Data, CollapseID: collapseID, Channel: payload.Channel,
		}, payload.NotificationID)
	}
}

// pushToUser sends msg to each of the user's phones. A token FCM answers UNREGISTERED is deleted; a
// refused message is logged under what and dropped; an outage is answered so the task retries.
func pushToUser(ctx context.Context, db coreServices.DatabaseService, pusher interfaces.Pusher, userID uuid.UUID, msg interfaces.PushMessage, what string) error {
	repo := trackingRepos.NewTrackingRepository(db)
	tokens, err := repo.PushTokens(ctx, userID)
	if err != nil {
		return err
	}
	var outage error
	for _, token := range tokens {
		msg.Token = token
		err := pusher.Send(ctx, msg)
		switch {
		case err == nil:
		case errors.Is(err, interfaces.ErrPushUnregistered):
			if err := repo.ForgetPushToken(ctx, token); err != nil {
				return err
			}
		case errors.Is(err, interfaces.ErrPushRejected):
			log.Printf("⚠️  push %s: %v", what, err)
		default:
			outage = err
		}
	}
	return outage
}
