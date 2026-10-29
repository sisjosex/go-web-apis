-- TRACK-071: an alert belongs to the tenant, a route is optional (one alert for the whole operation),
-- two kinds the server raises itself, one row of thresholds per tenant, the planned line as geography
-- for the off-route check, and the run of consecutive off-route fixes on the trip.

ALTER TABLE tracking.route_alerts ADD COLUMN tenant_id UUID;

UPDATE tracking.route_alerts a
SET tenant_id = tc.tenant_id
FROM tracking.routes r
INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
WHERE r.id = a.route_id;

ALTER TABLE tracking.route_alerts
    ALTER COLUMN tenant_id SET NOT NULL,
    ALTER COLUMN route_id DROP NOT NULL,
    DROP CONSTRAINT chk_alert_type,
    ADD CONSTRAINT chk_alert_type CHECK (alert_type IN
        ('delay', 'breakdown', 'cancellation', 'emergency', 'other', 'device_silent', 'off_route'));

COMMENT ON COLUMN tracking.route_alerts.route_id IS 'NULL: an alert to the whole operation (TRACK-071)';

CREATE INDEX idx_route_alerts_tenant_active ON tracking.route_alerts (tenant_id, created_at) WHERE status = 'active';

CREATE TABLE tracking.alert_settings (
    tenant_id         UUID PRIMARY KEY,
    device_silent_min SMALLINT NOT NULL DEFAULT 5   CONSTRAINT chk_alert_settings_silent CHECK (device_silent_min BETWEEN 1 AND 60),
    off_route_m       SMALLINT NOT NULL DEFAULT 150 CONSTRAINT chk_alert_settings_off_route_m CHECK (off_route_m BETWEEN 50 AND 1000),
    off_route_fixes   SMALLINT NOT NULL DEFAULT 3   CONSTRAINT chk_alert_settings_off_route_fixes CHECK (off_route_fixes BETWEEN 2 AND 10),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE tracking.alert_settings IS
'The thresholds of the two alerts the server raises (TRACK-071 D1): no position for device_silent_min during a trip; off_route_fixes fixes in a row farther than off_route_m from the planned line. No row: 5 / 150 / 3';

ALTER TABLE tracking.route_versions ADD COLUMN planned_line GEOGRAPHY(LineString, 4326);

COMMENT ON COLUMN tracking.route_versions.planned_line IS
'planned_polyline as geography, for the off-route check; NULL for a fallback (straight) line, which has no street to be off (TRACK-071)';

ALTER TABLE tracking.trips ADD COLUMN off_route_fixes SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tracking.trips.off_route_fixes IS
'Consecutive fixes off the planned line (positive) or back on it (negative): the hysteresis of the off_route alert (TRACK-071)';

-- notified_count of the alerts board: the delay notices one alert wrote, looked up by its id.
CREATE INDEX idx_notifications_delay_dedupe ON tracking.notifications (dedupe_key) WHERE type = 'delay';

-- The overloads without a tenant: every caller now passes it, and one may name no route.
DROP FUNCTION IF EXISTS tracking.sp_create_route_alert(uuid, uuid, character varying, character varying, text, uuid);
DROP FUNCTION IF EXISTS tracking.sp_create_route_alert(uuid, uuid, character varying, character varying, text, character varying, integer, uuid);
