package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
)

// topicAlertChanged is what raising and resolving an alert write to tracking.outbox (TRACK-004 D1).
const topicAlertChanged = "alert.changed"

// TaskAlertChanged is the outbox task one of those rows becomes.
var TaskAlertChanged = coreJobs.OutboxTask(topicAlertChanged)

// FrameAlert is the frame an alert.changed row is published as.
const FrameAlert = "alert"

// AlertChanged is one such row: which alert, on which route and trip (nil for an office alert), and
// whether it was raised or resolved.
type AlertChanged struct {
	AlertID string  `json:"alert_id"`
	RouteID string  `json:"route_id"`
	TripID  *string `json:"trip_id"`
	Type    string  `json:"type"`
}

// AlertChangedHandler consumes alert.changed: the row goes out as it is, as an `alert` frame, to the
// tenant's fleet — where the alerts page listens and refetches — and to the trip when it names one.
// The tenant comes from the TaskID. valkey nil publishes nothing. Delivery is at least once: a
// repeated frame only makes a client refetch.
func AlertChangedHandler(tenants coreJobs.TenantLister, valkey coreServices.ValkeyService) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload AlertChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("alert.changed payload: %v: %w", err, asynq.SkipRetry)
		}
		if valkey == nil {
			return nil
		}
		tenant, ok, err := taskTenant(ctx, tenants)
		if err != nil || !ok {
			return err
		}
		channels := []string{FleetChannel(tenant.ID)}
		if payload.TripID != nil {
			channels = append(channels, TripChannel(*payload.TripID))
		}
		pipe := valkey.Client().Pipeline()
		publishTo(ctx, pipe, channels, Frame{Type: FrameAlert, Data: task.Payload()})
		_, err = pipe.Exec(ctx)
		return err
	}
}
