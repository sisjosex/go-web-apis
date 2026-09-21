package jobs

import (
	"context"
	"encoding/json"
	"log"

	coreJobs "josex/web/modules/core/jobs"

	"github.com/hibiken/asynq"
)

// topicRouteChanged is what the stop-place and route-version SPs write to tracking.outbox whenever a
// route's stop list changes (TRACK-007 step 4).
const topicRouteChanged = "route.changed"

// TaskRouteChanged is the outbox task one of those rows becomes.
var TaskRouteChanged = coreJobs.OutboxTask(topicRouteChanged)

// RouteChanged is one such row: which route changed, and over which days. DateTo empty means "and
// every day after" — a published version has no end until the next one closes it.
type RouteChanged struct {
	RouteID  string `json:"route_id"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

// RouteChangedHandler is the TaskRouteChanged handler. Until TRACK-008 there are no trips to rebuild,
// so it only records that the change arrived — the producers write the rows now so that the consumer
// has a complete history to work from the day it exists. Delivery is at least once and this does
// nothing, so it is idempotent by construction.
func RouteChangedHandler() asynq.HandlerFunc {
	return func(_ context.Context, task *asynq.Task) error {
		var payload RouteChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		until := payload.DateTo
		if until == "" {
			until = "onwards"
		}
		log.Printf("📣 route.changed: route %s from %s to %s (no consumer until TRACK-008)",
			payload.RouteID, payload.DateFrom, until)
		return nil
	}
}
