-- TRACK-066: ETA from history and delay notices by threshold.
--
-- D1  One row per pair of consecutive arrived stops, written when the trip completes (arrival to
--     arrival, ISO weekday, 2-hour local band); the median is read through the index over the last
--     90 days and the daily partitions walk prunes older rows. The key is (trip, from stop), so a
--     replayed completion writes nothing and no surrogate id is needed.
-- D3  trip_stops.delay_notified_at is when the stop's late-ETA notice went out, once per trip and stop.
-- D4  Each client sets the minutes of delay that warrant a notice to its families.

CREATE TABLE tracking.stop_pair_samples (
    trip_id            UUID        NOT NULL REFERENCES tracking.trips(id) ON DELETE CASCADE,
    from_stop_place_id UUID        NOT NULL REFERENCES tracking.stop_places(id) ON DELETE CASCADE,
    to_stop_place_id   UUID        NOT NULL REFERENCES tracking.stop_places(id) ON DELETE CASCADE,
    tenant_id          UUID        NOT NULL,
    weekday            SMALLINT    NOT NULL,
    hour_band          SMALLINT    NOT NULL,
    duration_s         INT         NOT NULL,
    observed_at        TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (trip_id, from_stop_place_id),
    CONSTRAINT chk_stop_pair_samples_weekday CHECK (weekday BETWEEN 1 AND 7),
    CONSTRAINT chk_stop_pair_samples_hour_band CHECK (hour_band BETWEEN 0 AND 11),
    CONSTRAINT chk_stop_pair_samples_duration CHECK (duration_s > 0)
);

CREATE INDEX idx_stop_pair_samples_pair ON tracking.stop_pair_samples
    (tenant_id, from_stop_place_id, to_stop_place_id, weekday, hour_band, observed_at);

COMMENT ON TABLE tracking.stop_pair_samples IS
'Observed arrival-to-arrival seconds between consecutive stops of completed trips: the ETA source when the router is down (TRACK-066 D1)';

ALTER TABLE tracking.trip_stops ADD COLUMN delay_notified_at TIMESTAMPTZ;

COMMENT ON COLUMN tracking.trip_stops.delay_notified_at IS
'When the stop''s late-ETA notice went out: the first fresh ETA past the threshold of its riders'' clients (TRACK-066 D3)';

ALTER TABLE tracking.organizations
    ADD COLUMN delay_notice_min INT NOT NULL DEFAULT 10,
    ADD CONSTRAINT chk_organization_delay_notice CHECK (delay_notice_min BETWEEN 0 AND 180);

COMMENT ON COLUMN tracking.organizations.delay_notice_min IS
'Minutes of ETA past planned_at at a stop that send its pending riders'' families a delay notice (TRACK-066 D4)';
