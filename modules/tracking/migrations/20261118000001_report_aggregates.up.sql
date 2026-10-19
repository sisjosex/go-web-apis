-- TRACK-013: the reports read aggregates, not trips.
--
-- D1  One summary row per ended trip and one attendance row per rider of it, written by
--     sp_trip_summarise when the trip completes or is cancelled (outbox:trip.changed, after the trace
--     so distance_km is in) and again by the nightly sweep for whatever is missing or changed.
-- D5  stop_delay_stats is the p50/p90 lateness of each stop of a route by weekday; only the
--     punctuality report reads it.

CREATE TABLE tracking.trip_summaries (
    trip_id            UUID          PRIMARY KEY REFERENCES tracking.trips(id) ON DELETE CASCADE,
    tenant_id          UUID          NOT NULL,
    service_date       DATE          NOT NULL,
    route_id           UUID          NOT NULL,
    organization_id    UUID,
    organization_ids   UUID[]        NOT NULL DEFAULT '{}',
    vehicle_id         UUID,
    driver_id          UUID,
    status             VARCHAR(20)   NOT NULL,
    planned_start      TIMESTAMPTZ   NOT NULL,
    started_at         TIMESTAMPTZ,
    ended_at           TIMESTAMPTZ,
    distance_km        NUMERIC(8,2),
    duration_min       INT,
    tasks_planned      INT           NOT NULL DEFAULT 0,
    pickups_done       INT           NOT NULL DEFAULT 0,
    no_shows           INT           NOT NULL DEFAULT 0,
    cancelled          INT           NOT NULL DEFAULT 0,
    max_onboard        INT           NOT NULL DEFAULT 0,
    capacity           INT,
    start_delay_min    INT,
    avg_stop_delay_min NUMERIC(6,1),
    max_stop_delay_min INT,
    stops_late         INT           NOT NULL DEFAULT 0,
    alerts_count       INT           NOT NULL DEFAULT 0,
    rolled_at          TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT chk_trip_summaries_status CHECK (status IN ('completed', 'cancelled'))
);

CREATE INDEX idx_trip_summaries_tenant_date ON tracking.trip_summaries (tenant_id, service_date);
CREATE INDEX idx_trip_summaries_route_date ON tracking.trip_summaries (route_id, service_date);
CREATE INDEX idx_trip_summaries_vehicle_date ON tracking.trip_summaries (vehicle_id, service_date);
CREATE INDEX idx_trip_summaries_driver_date ON tracking.trip_summaries (driver_id, service_date);
CREATE INDEX idx_trip_summaries_organizations ON tracking.trip_summaries USING GIN (organization_ids);

COMMENT ON TABLE tracking.trip_summaries IS
'One row per ended trip: km, minutes, riders, lateness and alerts, rolled at trip close and by the nightly sweep (TRACK-013 D1)';
COMMENT ON COLUMN tracking.trip_summaries.organization_ids IS
'The organizations of the trip''s riders: what an organization user''s scope matches (TRACK-013 D4)';
COMMENT ON COLUMN tracking.trip_summaries.max_onboard IS
'The most riders aboard at once: done pickups minus done dropoffs, in done_at order';
COMMENT ON COLUMN tracking.trip_summaries.stops_late IS
'Stops arrived more than 5 minutes after planned_at';

CREATE TABLE tracking.rider_day_attendance (
    trip_id         UUID        NOT NULL REFERENCES tracking.trips(id) ON DELETE CASCADE,
    rider_id        UUID        NOT NULL,
    tenant_id       UUID        NOT NULL,
    service_date    DATE        NOT NULL,
    direction       VARCHAR(20) NOT NULL,
    route_id        UUID        NOT NULL,
    organization_id UUID        NOT NULL,
    status          VARCHAR(20) NOT NULL,
    PRIMARY KEY (trip_id, rider_id),
    CONSTRAINT chk_rider_day_attendance_status CHECK (status IN ('boarded', 'no_show', 'absent', 'cancelled'))
);

CREATE INDEX idx_rider_day_attendance_organization ON tracking.rider_day_attendance (organization_id, service_date);
CREATE INDEX idx_rider_day_attendance_rider ON tracking.rider_day_attendance (rider_id, service_date);
CREATE INDEX idx_rider_day_attendance_tenant ON tracking.rider_day_attendance (tenant_id, service_date);

COMMENT ON TABLE tracking.rider_day_attendance IS
'Each rider''s pickup on an ended trip: boarded, no_show, absent (an absence covered the day) or cancelled (TRACK-013 D1)';

CREATE TABLE tracking.stop_delay_stats (
    route_id      UUID         NOT NULL REFERENCES tracking.routes(id) ON DELETE CASCADE,
    stop_place_id UUID         NOT NULL REFERENCES tracking.stop_places(id) ON DELETE CASCADE,
    weekday       SMALLINT     NOT NULL,
    tenant_id     UUID         NOT NULL,
    samples       INT          NOT NULL,
    p50_delay_min NUMERIC(6,1) NOT NULL,
    p90_delay_min NUMERIC(6,1) NOT NULL,
    computed_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (route_id, stop_place_id, weekday),
    CONSTRAINT chk_stop_delay_stats_weekday CHECK (weekday BETWEEN 1 AND 7)
);

CREATE INDEX idx_stop_delay_stats_tenant ON tracking.stop_delay_stats (tenant_id);

COMMENT ON TABLE tracking.stop_delay_stats IS
'Minutes each stop of a route is reached past planned_at, p50 and p90 by ISO weekday, over completed trips of the last weeks (TRACK-013 D5)';
