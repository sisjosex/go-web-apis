-- TRACK-013 D3: what each tenant used, one row per day.
--
-- The nightly tracking:report-rollups pass counts yesterday (UTC) in the tenant's own database and
-- writes it here, in the platform database, so the platform reads every tenant's usage in one query.
-- A vehicle is active on a day when it drove a trip that ended or sent a GPS point: per-vehicle
-- billing pays for what runs, not what is registered. The billing rule over these rows is not here.

CREATE TABLE tenancy.usage_daily (
    tenant_id       UUID        NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    day             DATE        NOT NULL,
    vehicles_active INT         NOT NULL DEFAULT 0,
    trips_completed INT         NOT NULL DEFAULT 0,
    positions       BIGINT      NOT NULL DEFAULT 0,
    riders_boarded  INT         NOT NULL DEFAULT 0,
    rolled_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, day)
);

CREATE INDEX idx_usage_daily_day ON tenancy.usage_daily (day);

COMMENT ON TABLE tenancy.usage_daily IS
'Per tenant and UTC day: vehicles that ran, trips completed, GPS points and riders boarded (TRACK-013 D3)';
