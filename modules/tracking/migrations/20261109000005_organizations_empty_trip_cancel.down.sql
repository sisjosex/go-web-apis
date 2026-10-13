-- TRACK-069 down: no rider_added notice, no per-client auto-cancel lead.

DELETE FROM tracking.notification_settings WHERE type = 'rider_added';
DELETE FROM tracking.notifications WHERE type = 'rider_added';

ALTER TABLE tracking.notification_settings
    DROP CONSTRAINT chk_notification_settings_type,
    ADD CONSTRAINT chk_notification_settings_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay',
        'trip_progress'
    ));

ALTER TABLE tracking.notifications
    DROP CONSTRAINT chk_notifications_type,
    ADD CONSTRAINT chk_notifications_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay'
    ));

ALTER TABLE tracking.organizations
    DROP CONSTRAINT chk_organization_empty_trip_cancel,
    DROP COLUMN empty_trip_cancel_min;
