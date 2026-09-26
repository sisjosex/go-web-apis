DROP FUNCTION IF EXISTS tracking.sp_driver_incident(UUID, UUID, UUID, VARCHAR, TEXT, DOUBLE PRECISION, DOUBLE PRECISION);
DROP FUNCTION IF EXISTS tracking.sp_driver_find_riders(UUID, UUID, TEXT);
DROP FUNCTION IF EXISTS tracking.sp_driver_resolve_rider(UUID, UUID, TEXT);
DROP FUNCTION IF EXISTS tracking.fn_driver_pending_pickup(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_rider_qr_token(UUID, UUID, BOOLEAN, UUID);

DROP INDEX IF EXISTS tracking.idx_route_alerts_trip_id;
ALTER TABLE tracking.route_alerts
    DROP COLUMN IF EXISTS location,
    DROP COLUMN IF EXISTS trip_id;

ALTER TABLE tracking.riders DROP COLUMN IF EXISTS qr_token;

DROP FUNCTION IF EXISTS tracking.fn_new_qr_token();
