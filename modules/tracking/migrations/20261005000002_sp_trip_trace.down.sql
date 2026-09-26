DROP FUNCTION IF EXISTS tracking.sp_get_trip_traced(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_trip_close_trace(UUID, UUID, TEXT, NUMERIC, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_trip_trace_points(UUID, UUID);
ALTER TABLE tracking.trips
    DROP COLUMN IF EXISTS trace_source,
    DROP COLUMN IF EXISTS distance_km,
    DROP COLUMN IF EXISTS polyline;
