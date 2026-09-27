-- TRACK-012 step 1: the in-app feed and what a guardian muted.
--
-- notifications is the durable record of every notice (D1): the push is only a copy of a row, so a
-- phone that was off, a token FCM dropped or FCM itself down still leaves the notice here. One row per
-- recipient and rider: a guardian of two riders on one trip gets two rows. The UNIQUE key is what
-- makes the outbox's at-least-once delivery write each notice once — a replayed event inserts nothing.
--
-- notification_settings holds the muted pairs only: no row is "notify", so a new rider or a new type
-- is on by default and the table stays as small as what people actually turned off. A muted type
-- still gets its feed row; only the push is skipped.
--
-- user_id has no foreign key: auth.users lives in the main database, not in the tenant's.

CREATE TABLE tracking.notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    rider_id    UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    type        VARCHAR(20) NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at     TIMESTAMPTZ,
    CONSTRAINT chk_notifications_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay'
    )),
    CONSTRAINT ux_notifications_dedupe UNIQUE (user_id, rider_id, type, dedupe_key)
);

-- The feed reads one user's rows newest first; the badge counts the unread slice.
CREATE INDEX idx_notifications_user ON tracking.notifications (user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON tracking.notifications (user_id) WHERE read_at IS NULL;

COMMENT ON TABLE tracking.notifications IS
'The in-app feed: one row per notice per recipient and rider, written by sp_notify_event; the push is a copy of a row (TRACK-012)';

CREATE TABLE tracking.notification_settings (
    user_id  UUID NOT NULL,
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    type     VARCHAR(20) NOT NULL,
    PRIMARY KEY (user_id, rider_id, type),
    CONSTRAINT chk_notification_settings_type CHECK (type IN (
        'trip_started', 'approaching', 'boarded', 'dropped_off', 'no_show', 'cancelled', 'changed', 'delay'
    ))
);

COMMENT ON TABLE tracking.notification_settings IS
'The (user, rider, type) pushes a recipient muted; no row is on. A muted notice keeps its feed row (TRACK-012)';
