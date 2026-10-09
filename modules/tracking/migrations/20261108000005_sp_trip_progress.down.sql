-- MOBILE-024 down: no trip-progress read, and trip_progress no longer a mutable type.
DROP FUNCTION IF EXISTS tracking.sp_trip_progress(UUID, UUID);

DELETE FROM tracking.notification_settings WHERE type = 'trip_progress';

ALTER TABLE tracking.notification_settings
    DROP CONSTRAINT chk_notification_settings_type,
    ADD CONSTRAINT chk_notification_settings_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay'
    ));
