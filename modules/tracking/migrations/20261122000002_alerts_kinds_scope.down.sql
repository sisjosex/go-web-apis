DROP INDEX IF EXISTS tracking.idx_notifications_delay_dedupe;
ALTER TABLE tracking.trips DROP COLUMN off_route_fixes;
ALTER TABLE tracking.route_versions DROP COLUMN planned_line;
DROP TABLE tracking.alert_settings;
DROP INDEX IF EXISTS tracking.idx_route_alerts_tenant_active;
DELETE FROM tracking.route_alerts WHERE route_id IS NULL OR alert_type IN ('device_silent', 'off_route');
ALTER TABLE tracking.route_alerts
    DROP CONSTRAINT chk_alert_type,
    ADD CONSTRAINT chk_alert_type CHECK (alert_type IN ('delay', 'breakdown', 'cancellation', 'emergency', 'other')),
    ALTER COLUMN route_id SET NOT NULL,
    DROP COLUMN tenant_id;
