package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingConfig "josex/web/modules/tracking/config"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Notifier turns an outbox event into the guardians' notices (TRACK-012 step 4): one sp_notify_event
// per event writes the feed rows, and each new row whose type its account did not mute is enqueued
// as a push-send. A nil Notifier notifies nothing; a nil client writes the feed only (no FCM).
type Notifier struct {
	db          coreServices.DatabaseService
	client      *asynq.Client
	tripEvents  bool
	alertEvents bool
}

// NewNotifier gates trip events on TRACKING_ENABLE_REALTIME_NOTIFICATIONS and alert events on
// TRACKING_ENABLE_ALERT_NOTIFICATIONS.
func NewNotifier(db coreServices.DatabaseService, client *asynq.Client, cfg *trackingConfig.TrackingConfig) *Notifier {
	return &Notifier{db: db, client: client, tripEvents: cfg.EnableRealtimeNotifications, alertEvents: cfg.EnableAlertNotifications}
}

// Notify handles one event of topic for tenant. The outbox row's id, from the task's TaskID
// <tenant>:<id>, is the dedupe key of an event that has no natural one. Delivery is at least once: a
// replayed event writes no feed row, and a push-send already enqueued keeps its TaskID.
func (n *Notifier) Notify(ctx context.Context, tenant coreJobs.Tenant, topic string, event []byte) error {
	if n == nil || (topic == topicTripChanged && !n.tripEvents) || (topic == topicAlertChanged && !n.alertEvents) {
		return nil
	}
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	var eventID *int64
	taskID, _ := asynq.GetTaskID(ctx)
	if _, id, ok := strings.Cut(taskID, ":"); ok {
		if parsed, err := strconv.ParseInt(id, 10, 64); err == nil {
			eventID = &parsed
		}
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	notices, err := trackingRepos.NewTrackingRepository(n.db).NotifyEvent(tenantCtx, tenantID, topic, event, eventID)
	if err != nil || n.client == nil {
		return err
	}
	for _, notice := range notices {
		if err := n.enqueue(ctx, notice); err != nil {
			return err
		}
	}
	return nil
}

func (n *Notifier) enqueue(ctx context.Context, notice models.NewNotice) error {
	var body map[string]any
	_ = json.Unmarshal(notice.Payload, &body)
	data := map[string]string{"notification_id": notice.ID.String(), "rider_id": notice.RiderID.String(), "type": notice.Type}
	if trip, ok := body["trip_id"].(string); ok {
		data["trip_id"] = trip
	}
	payload, err := json.Marshal(PushSend{
		NotificationID: notice.ID.String(), UserID: notice.UserID.String(), Type: notice.Type, Args: pushArgs(body), Data: data,
	})
	if err != nil {
		return err
	}
	_, err = n.client.EnqueueContext(ctx, asynq.NewTask(TaskPushSend, payload),
		asynq.TaskID(notice.ID.String()), asynq.MaxRetry(pushSendRetries), asynq.Queue(coreJobs.QueueCritical))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// pushArgs are the body's placeholders, in the order every notice string takes them: the rider, the
// route, then the stop or the alert's title when the notice has one.
func pushArgs(body map[string]any) []string {
	args := []string{}
	for _, key := range []string{"rider_name", "route_name", "stop_name", "title"} {
		if v, ok := body[key].(string); ok {
			args = append(args, v)
		}
	}
	return args
}
