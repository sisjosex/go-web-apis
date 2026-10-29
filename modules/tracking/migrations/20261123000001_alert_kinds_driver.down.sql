DELETE FROM tracking.route_alerts WHERE alert_type IN ('road_closed', 'passenger', 'accident');

ALTER TABLE tracking.route_alerts
    DROP COLUMN notify_families,
    DROP CONSTRAINT chk_alert_type,
    ADD CONSTRAINT chk_alert_type CHECK (alert_type IN
        ('delay', 'breakdown', 'cancellation', 'emergency', 'other', 'device_silent', 'off_route'));
