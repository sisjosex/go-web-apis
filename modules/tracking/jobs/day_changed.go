package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// TaskDayChanged tells one driver its day changed (TRACK-030 D3): a day_changed frame on its channel
// and a silent push to its phones, so the app re-reads its day — a 304 when nothing it shows moved.
const TaskDayChanged = "tracking:day-changed"

// FrameDayChanged is the driver channel's one frame; it carries no data.
const FrameDayChanged = "day_changed"

// dayChangedAfter groups a driver's signals: the first change of a burst enqueues the task this far
// out, under a TaskID the next ones collide with, so one signal follows the burst's last change.
const dayChangedAfter = 2 * time.Second

// DriverChannel is a driver's own channel; only the user linked to that driver may listen.
func DriverChannel(driverID string) string { return "driver:" + driverID }

// DayChanged is the task's payload: everything it needs, so sending reads nothing of the tenant.
type DayChanged struct {
	TenantSlug string `json:"tenant_slug"`
	DriverID   string `json:"driver_id"`
	UserID     string `json:"user_id,omitempty"`
}

// DriverSignal enqueues day_changed for the drivers a change touches. A nil DriverSignal signals
// nothing.
type DriverSignal struct {
	db     coreServices.DatabaseService
	client *asynq.Client
}

// NewDriverSignal builds the signal over the jobs client.
func NewDriverSignal(db coreServices.DatabaseService, client *asynq.Client) *DriverSignal {
	return &DriverSignal{db: db, client: client}
}

// Signal enqueues one grouped day_changed per driver with a trip today on routeID (within from..to)
// or on tripID. One sp_driver_audience per event; a driver already waiting for its signal enqueues
// nothing new.
func (s *DriverSignal) Signal(ctx context.Context, tenant coreJobs.Tenant, routeID, tripID *uuid.UUID, from, to *time.Time) error {
	if s == nil {
		return nil
	}
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	drivers, err := trackingRepos.NewTrackingRepository(s.db).DriverAudience(tenantCtx, tenantID, routeID, tripID, from, to)
	if err != nil {
		return err
	}
	for _, d := range drivers {
		payload := DayChanged{TenantSlug: tenant.Slug, DriverID: d.DriverID.String()}
		if d.UserID != nil {
			payload.UserID = d.UserID.String()
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = s.client.EnqueueContext(ctx, asynq.NewTask(TaskDayChanged, raw),
			asynq.TaskID("day:"+tenant.ID+":"+payload.DriverID), asynq.ProcessIn(dayChangedAfter),
			asynq.MaxRetry(3), asynq.Queue(coreJobs.QueueCritical))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			return err
		}
	}
	return nil
}

// DayChangedHandler publishes the frame and, with a pusher and a linked user, sends the silent push
// — collapsed per driver, so a phone offline for a while gets only the newest. valkey nil publishes
// nothing. A push outage fails the task: the retry repeats the frame, which only makes the app
// re-read.
func DayChangedHandler(db coreServices.DatabaseService, valkey coreServices.ValkeyService, pusher interfaces.Pusher) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload DayChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("day-changed payload: %v: %w", err, asynq.SkipRetry)
		}
		if valkey != nil {
			frame, err := json.Marshal(Frame{Type: FrameDayChanged})
			if err != nil {
				return err
			}
			if err := valkey.Client().Publish(ctx, DriverChannel(payload.DriverID), frame).Err(); err != nil {
				return err
			}
		}
		userID, err := uuid.Parse(payload.UserID)
		if pusher == nil || err != nil {
			return nil
		}
		return pushToUser(ctx, db, pusher, userID, interfaces.PushMessage{
			Data:       map[string]string{"type": FrameDayChanged, "tenant_slug": payload.TenantSlug},
			CollapseID: "day:" + payload.DriverID,
			Silent:     true,
		}, "day-changed "+payload.DriverID)
	}
}
