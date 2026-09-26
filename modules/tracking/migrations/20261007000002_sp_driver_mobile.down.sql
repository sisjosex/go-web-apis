-- Reverses TRACK-011 step 2: the driver's SPs. driver_ops rows they wrote stay until step 1 is reversed.

DROP FUNCTION IF EXISTS tracking.sp_driver_sync(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_driver_start_trip(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_driver_today(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_driver_of_user(UUID, UUID);
