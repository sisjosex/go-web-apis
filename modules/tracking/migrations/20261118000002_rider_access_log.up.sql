-- TRACK-063: who outside the operator read a rider, and when a rider stopped riding.
--
-- D1  One row per person, rider, action and hour, written by the read itself (fn_record_rider_access):
--     a guardian refreshing every 30 s leaves one row an hour with reads counting the calls.
-- D2  Only the portal and organization levels are logged; the operator's own staff leave no row.
-- D3  deactivated_at starts the contact retention clock; anonymised_at marks the pass that cleared them.

CREATE TABLE tracking.rider_access_log (
    id        BIGSERIAL    PRIMARY KEY,
    tenant_id UUID         NOT NULL,
    -- No foreign key: the trail outlives the rider.
    rider_id  UUID         NOT NULL,
    user_id   UUID         NOT NULL,
    level     VARCHAR(20)  NOT NULL,
    action    VARCHAR(20)  NOT NULL,
    bucket    TIMESTAMPTZ  NOT NULL,
    reads     INT          NOT NULL DEFAULT 1,
    first_at  TIMESTAMPTZ  NOT NULL,
    last_at   TIMESTAMPTZ  NOT NULL,
    CONSTRAINT chk_rider_access_log_level CHECK (level IN ('portal', 'organization')),
    CONSTRAINT chk_rider_access_log_action CHECK (action IN ('status', 'today', 'live', 'absences')),
    CONSTRAINT uq_rider_access_log_bucket UNIQUE (rider_id, user_id, action, bucket)
);

CREATE INDEX idx_rider_access_log_rider_last ON tracking.rider_access_log (rider_id, last_at DESC);
CREATE INDEX idx_rider_access_log_tenant_last ON tracking.rider_access_log (tenant_id, last_at);

COMMENT ON TABLE tracking.rider_access_log IS
'Reads of a rider by a guardian (portal) or a client''s user (organization), one row per user, rider, action and hour; reads counts the calls in that hour (TRACK-063 D1, D2)';

ALTER TABLE tracking.riders
    ADD COLUMN deactivated_at TIMESTAMPTZ,
    ADD COLUMN anonymised_at  TIMESTAMPTZ;

UPDATE tracking.riders SET deactivated_at = updated_at WHERE status = 'inactive';

COMMENT ON COLUMN tracking.riders.deactivated_at IS
'When the rider last turned inactive, NULL while active; the contact retention counts from here (TRACK-063 D3)';
COMMENT ON COLUMN tracking.riders.anonymised_at IS
'When the retention pass cleared the rider''s contact data (TRACK-063 D3)';
