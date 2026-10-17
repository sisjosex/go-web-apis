-- TRACK-060: share links — a time-boxed public link to a trip.
--
-- D1  rider_id NULL is the operator's link (the whole trip, no names); set, it is a guardian's link
--     (that rider's stops and first name only).
-- The token is <slug>.<secret>; only its sha256 is kept, so a leaked table hands out no link.

CREATE TABLE tracking.share_links (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    trip_id     UUID        NOT NULL REFERENCES tracking.trips(id) ON DELETE CASCADE,
    rider_id    UUID        REFERENCES tracking.riders(id) ON DELETE CASCADE,
    token_hash  BYTEA       NOT NULL UNIQUE,
    created_by  UUID,
    created_via VARCHAR(10) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_share_links_via CHECK (created_via IN ('web', 'portal'))
);

CREATE INDEX idx_share_links_trip ON tracking.share_links (trip_id);

COMMENT ON TABLE tracking.share_links IS
'Public links to a trip: the sha256 of each token, who made it and until when it opens (TRACK-060)';
