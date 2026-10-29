package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"

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
// tenant's fleet — where the alerts page listens and refetches — and to the trip when it names one,
// with the rail's new counts on nav:{tenant} (TRACK-081).
// A raised alert is also a `delay` notice to the guardians of the riders it touches (TRACK-012), and a
// serious one a push to the operator's staff (TRACK-071 D3).
// The tenant comes from the TaskID. valkey nil publishes nothing; notify nil notifies nothing; pusher
// or staff nil pushes nothing to the staff.
// Delivery is at least once: a repeated frame only makes a client refetch, a repeated notice writes
// nothing, a repeated push replaces the one shown (its collapse id).
func AlertChangedHandler(db coreServices.DatabaseService, tenants *coreJobs.TenantDirectory, valkey coreServices.ValkeyService, notify *Notifier, pusher interfaces.Pusher, staff StaffDirectory) asynq.HandlerFunc {
	return func(ctx context.Context, task *asynq.Task) error {
		var payload AlertChanged
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return fmt.Errorf("alert.changed payload: %v: %w", err, asynq.SkipRetry)
		}
		if valkey == nil && notify == nil && pusher == nil {
			return nil
		}
		tenant, ok, err := taskTenant(ctx, tenants)
		if err != nil || !ok {
			return err
		}
		if err := publishAlert(ctx, db, valkey, tenant, payload, task.Payload()); err != nil {
			return err
		}
		if err := notify.Notify(ctx, tenant, topicAlertChanged, task.Payload()); err != nil {
			return err
		}
		if payload.Type != "raised" || pusher == nil || staff == nil {
			return nil
		}
		alertID, err := uuid.Parse(payload.AlertID)
		if err != nil {
			return fmt.Errorf("alert.changed alert_id: %v: %w", err, asynq.SkipRetry)
		}
		return PushStaffAlert(ctx, db, tenant, alertID, pusher, staff)
	}
}

// publishAlert sends the row as an `alert` frame to the tenant's fleet and the trip it names, with the
// rail's new counts; valkey nil publishes nothing.
func publishAlert(ctx context.Context, db coreServices.DatabaseService, valkey coreServices.ValkeyService, tenant coreJobs.Tenant, payload AlertChanged, raw []byte) error {
	if valkey == nil {
		return nil
	}
	channels := []string{FleetChannel(tenant.ID)}
	if payload.TripID != nil {
		channels = append(channels, TripChannel(*payload.TripID))
	}
	pipe := valkey.Client().Pipeline()
	publishTo(ctx, pipe, channels, Frame{Type: FrameAlert, Data: raw})
	if err := queueNavCounts(ctx, db, pipe, tenant); err != nil {
		return err
	}
	_, err := pipe.Exec(ctx)
	return err
}

// StaffDirectory answers the accounts of a tenant's staff — owner, admin, member (TRACK-071 D3).
// cmd/server builds it from the tenancy module, which tracking does not import.
type StaffDirectory func(ctx context.Context, tenantID uuid.UUID) ([]uuid.UUID, error)

// staffAlertKinds are the alerts serious enough to wake the staff's phones (TRACK-071 D3).
var staffAlertKinds = map[string]bool{"breakdown": true, "emergency": true, "device_silent": true, "off_route": true}

// PushStaffAlert pushes a raised alert of a serious kind to every staff account but the one that
// raised it, on the "problems" channel: `notice.staff_<kind>` with [what, route, plate], and the data
// the phone opens its Alertas list with. One platform read for the staff, one tenant read for the alert.
func PushStaffAlert(ctx context.Context, db coreServices.DatabaseService, tenant coreJobs.Tenant, alertID uuid.UUID, pusher interfaces.Pusher, staff StaffDirectory) error {
	tenantID, err := uuid.Parse(tenant.ID)
	if err != nil {
		return err
	}
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
	alert, err := trackingRepos.NewTrackingRepository(db).RouteAlert(tenantCtx, tenantID, alertID)
	if err != nil || alert == nil || !staffAlertKinds[alert.AlertType] {
		return err
	}
	users, err := staff(ctx, tenantID)
	if err != nil {
		return err
	}
	msg := interfaces.PushMessage{
		Type:       "staff_" + alert.AlertType,
		Args:       []string{staffAlertDetail(alert), orDash(alert.RouteName), orDash(alert.LicensePlate)},
		Data:       map[string]string{"type": "staff_alert", "alert_id": alert.ID.String(), "tenant_slug": tenant.Slug},
		CollapseID: "alert:" + alert.ID.String(),
		Channel:    "problems",
	}
	var outage error
	for _, userID := range users {
		if alert.CreatedBy != nil && *alert.CreatedBy == userID {
			continue
		}
		// The push tokens live in the platform database: the plain context, not the tenant's.
		if err := pushToUser(ctx, db, pusher, userID, msg, "staff alert "+alert.ID.String()); err != nil {
			outage = err
		}
	}
	return outage
}

// staffAlertDetail is the first argument: the minutes a bus is silent, else what the alert says.
func staffAlertDetail(alert *models.RouteAlert) string {
	if alert.AlertType == "device_silent" && alert.Message != nil {
		return *alert.Message + " min"
	}
	if alert.Message != nil && *alert.Message != "" {
		return *alert.Message
	}
	return alert.Title
}

func orDash(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}
