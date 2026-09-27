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
		if err := n.enqueue(ctx, tenant.Slug, notice); err != nil {
			return err
		}
	}
	return nil
}

// enqueue pushes one feed row. tenantSlug rides in the data so a tap selects the tenant before the
// rider; a trip's later notice for the same rider replaces the earlier one on the phone (D4).
func (n *Notifier) enqueue(ctx context.Context, tenantSlug string, notice models.NewNotice) error {
	var body map[string]any
	_ = json.Unmarshal(notice.Payload, &body)
	data := map[string]string{
		"notification_id": notice.ID.String(), "rider_id": notice.RiderID.String(), "type": notice.Type, "tenant_slug": tenantSlug,
	}
	collapseID := notice.ID.String()
	if trip, ok := body["trip_id"].(string); ok {
		data["trip_id"] = trip
		collapseID = tripCollapseID(trip, notice.RiderID)
	}
	payload, err := json.Marshal(PushSend{
		NotificationID: notice.ID.String(), UserID: notice.UserID.String(), Type: notice.Type, Args: pushArgs(body), Data: data,
		CollapseID: collapseID, Channel: noticeChannel(notice.Type),
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

// tripCollapseID names one rider's notices of one trip: the same for each, so the next replaces the
// last. A UUID v5 of the rider in the trip's namespace, because the plain `<trip>:<rider>` is 73 bytes
// and APNs refuses an apns-collapse-id over 64 — FCM then rejects the message for Android too.
func tripCollapseID(tripID string, riderID uuid.UUID) string {
	trip, err := uuid.Parse(tripID)
	if err != nil {
		return riderID.String()
	}
	return uuid.NewSHA1(trip, riderID[:]).String()
}

// noticeChannel is the Android channel a type shows on — the three the phone creates, so a guardian
// can silence one kind of notice from the system settings.
func noticeChannel(noticeType string) string {
	switch noticeType {
	case "trip_started", "boarded", "dropped_off":
		return "trip"
	case "approaching":
		return "approaching"
	default:
		return "problems"
	}
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
