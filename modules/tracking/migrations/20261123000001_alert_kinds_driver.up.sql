-- MOBILE-037: the three kinds only a driver reports (D1) beside the office's and the server's, and
-- whether a driver's incident tells the families and the client (D2). An office alert keeps the default.

ALTER TABLE tracking.route_alerts
    DROP CONSTRAINT chk_alert_type,
    ADD CONSTRAINT chk_alert_type CHECK (alert_type IN
        ('delay', 'breakdown', 'cancellation', 'emergency', 'other', 'device_silent', 'off_route',
         'road_closed', 'passenger', 'accident')),
    ADD COLUMN notify_families BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN tracking.route_alerts.notify_families IS
'FALSE: the driver switched "Avisar a padres y colegio" off — the office sees the alert, no notice or client push goes out (MOBILE-037 D2)';
