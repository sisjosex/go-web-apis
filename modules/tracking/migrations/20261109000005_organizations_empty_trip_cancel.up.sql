-- TRACK-069: trips hygiene.
--
-- D1  A client decides how many minutes before departure a trip still without riders cancels itself;
--     0 never. sp_cancel_empty_trips reads it every 5 minutes, one query per tenant.
-- D3  A rider added to a running trip is announced to the family as its own notice, rider_added,
--     which a guardian can mute per rider like the others.

ALTER TABLE tracking.organizations
    ADD COLUMN empty_trip_cancel_min INT NOT NULL DEFAULT 30,
    ADD CONSTRAINT chk_organization_empty_trip_cancel CHECK (empty_trip_cancel_min BETWEEN 0 AND 1440);

COMMENT ON COLUMN tracking.organizations.empty_trip_cancel_min IS
'Minutes before planned_start a trip of this client with no rider cancels itself; 0 never (TRACK-069 D1)';

ALTER TABLE tracking.notifications
    DROP CONSTRAINT chk_notifications_type,
    ADD CONSTRAINT chk_notifications_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay',
        'rider_added'
    ));

ALTER TABLE tracking.notification_settings
    DROP CONSTRAINT chk_notification_settings_type,
    ADD CONSTRAINT chk_notification_settings_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay',
        'trip_progress', 'rider_added'
    ));
