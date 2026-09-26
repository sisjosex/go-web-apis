package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	coreJobs "josex/web/modules/core/jobs"

	"github.com/hibiken/asynq"
)

// topicTripChanged is what every trip, stop and task transition writes to tracking.outbox (TRACK-020).
const topicTripChanged = "trip.changed"

// TaskTripChanged is the outbox task one of those rows becomes.
var TaskTripChanged = coreJobs.OutboxTask(topicTripChanged)

// TripChanged is one such row: which trip moved, on which route and day, and how — the action, or
// "override".
type TripChanged struct {
	TripID      string `json:"trip_id"`
	RouteID     string `json:"route_id"`
	ServiceDate string `json:"service_date"`
	Type        string `json:"type"`
}

// TripChangedHandler acknowledges each trip.changed task and does nothing else yet (D1): the row is
// written from the first transition so the event is durable and the relay proven with real data, and
// TRACK-010 (WebSocket) and TRACK-012 (push) replace this handler without touching the SPs.
func TripChangedHandler() asynq.HandlerFunc {
	return func(_ context.Context, task *asynq.Task) error {
		var payload TripChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("trip.changed payload: %v: %w", err, asynq.SkipRetry)
		}
		log.Printf("trip.changed %s %s acknowledged, no consumer yet", payload.TripID, payload.Type)
		return nil
	}
}
