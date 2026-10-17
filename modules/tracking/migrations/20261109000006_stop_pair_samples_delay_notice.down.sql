-- TRACK-066 down: no stop-to-stop history, no late-ETA notice.

ALTER TABLE tracking.organizations
    DROP CONSTRAINT chk_organization_delay_notice,
    DROP COLUMN delay_notice_min;

ALTER TABLE tracking.trip_stops DROP COLUMN delay_notified_at;

DROP TABLE tracking.stop_pair_samples;
