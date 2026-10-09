package repositories

import (
	"context"
	"encoding/json"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The portal's notices of TRACK-012: one SP per endpoint, the account's scope resolved inside each.

// ListNotifications answers one page of the account's feed with its total and unread badge.
func (r *TrackingRepository) ListNotifications(ctx context.Context, tenantID, userID uuid.UUID, query models.ListNotificationsQuery) (*models.ListNotificationsResponse, error) {
	var raw []byte
	err := r.dbService.QueryRow(ctx, `
		SELECT tracking.sp_list_notifications(p_tenant_id := $1, p_user_id := $2, p_unread := $3, p_page := $4, p_page_size := $5)
	`, tenantID, userID, query.Unread, query.Page, query.PageSize).Scan(&raw)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.NotificationListFailed, Err: err}
	}
	result := models.ListNotificationsResponse{Page: query.Page, PageSize: query.PageSize}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.NotificationListFailed, Err: err}
	}
	return &result, nil
}

// MarkNotificationsRead stamps the account's rows read; ids of someone else's rows change nothing.
func (r *TrackingRepository) MarkNotificationsRead(ctx context.Context, tenantID, userID uuid.UUID, dto models.MarkNotificationsReadDto) error {
	ids := dto.IDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	_, err := r.dbService.Execute(ctx, `
		SELECT tracking.sp_mark_notifications_read(p_tenant_id := $1, p_user_id := $2, p_ids := $3, p_all := $4)
	`, tenantID, userID, ids, dto.All)
	if err != nil {
		return &trackingErrors.TrackingError{Code: trackingErrors.NotificationReadFailed, Err: err}
	}
	return nil
}

// NotificationSettings answers each rider the account receives notices for, with what it muted.
func (r *TrackingRepository) NotificationSettings(ctx context.Context, tenantID, userID uuid.UUID) ([]models.NotificationSetting, error) {
	return r.queryNotificationSettings(ctx, `
		SELECT rider_id, rider_name, muted FROM tracking.sp_get_notification_settings(p_tenant_id := $1, p_user_id := $2)
	`, tenantID, userID)
}

// SetNotificationSettings replaces the muted types of each rider settings names; a rider outside the
// account's scope is rider.not-found and nothing is written.
func (r *TrackingRepository) SetNotificationSettings(ctx context.Context, tenantID, userID uuid.UUID, settings []byte) ([]models.NotificationSetting, error) {
	return r.queryNotificationSettings(ctx, `
		SELECT rider_id, rider_name, muted
		FROM tracking.sp_set_notification_settings(p_tenant_id := $1, p_user_id := $2, p_settings := $3::JSONB)
	`, tenantID, userID, settings)
}

func (r *TrackingRepository) queryNotificationSettings(ctx context.Context, query string, args ...any) ([]models.NotificationSetting, error) {
	rows, err := r.dbService.Query(ctx, query, args...)
	if err != nil {
		return nil, scopedErr(err, trackingErrors.NotificationSettingsFailed)
	}
	settings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.NotificationSetting, error) {
		var s models.NotificationSetting
		err := row.Scan(&s.RiderID, &s.RiderName, &s.Muted)
		return s, err
	})
	if err != nil {
		return nil, scopedErr(err, trackingErrors.NotificationSettingsFailed)
	}
	return settings, nil
}

// The push task's reads of TRACK-012 D3: auth.push_devices lives in the main database, so these run
// on a context that names no tenant, whatever tenant the notice came from.

// TripProgress answers what each guardian's trip-progress push says (MOBILE-024): one row per rider with
// a task on the trip and guardian who did not mute it; none while the trip is planned.
func (r *TrackingRepository) TripProgress(ctx context.Context, tenantID, tripID uuid.UUID) ([]models.TripProgressRow, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_trip_progress(p_tenant_id := $1, p_trip_id := $2)`, tenantID, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.TripProgressRow{}
	for rows.Next() {
		var p models.TripProgressRow
		if err := rows.Scan(&p.RiderID, &p.RiderName, &p.UserID, &p.Phase, &p.StopsDone, &p.StopsTotal, &p.CurrentStop,
			&p.NextStop, &p.TargetStop, &p.TargetTripStopID, &p.TripStatus, &p.VehicleType, &p.OrganizationKind); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PushTokens answers every push token of an account.
func (r *TrackingRepository) PushTokens(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.dbService.Query(ctx, `SELECT token FROM auth.sp_push_tokens(p_user_id := $1)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ForgetPushToken deletes a token FCM answered UNREGISTERED.
func (r *TrackingRepository) ForgetPushToken(ctx context.Context, token string) error {
	_, err := r.dbService.Execute(ctx, `SELECT auth.sp_forget_push_token(p_token := $1)`, token)
	return err
}

// NotifyEvent turns one outbox event into feed rows and answers the ones to push (TRACK-012 step 4).
// eventID is the outbox row's id: the dedupe key of an event that has no natural one.
func (r *TrackingRepository) NotifyEvent(ctx context.Context, tenantID uuid.UUID, topic string, event []byte, eventID *int64) ([]models.NewNotice, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT id, user_id, rider_id, type, payload
		FROM tracking.sp_notify_event(p_tenant_id := $1, p_topic := $2, p_event := $3::JSONB, p_event_id := $4)
	`, tenantID, topic, event, eventID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.NewNotice, error) {
		var n models.NewNotice
		err := row.Scan(&n.ID, &n.UserID, &n.RiderID, &n.Type, &n.Payload)
		return n, err
	})
}
