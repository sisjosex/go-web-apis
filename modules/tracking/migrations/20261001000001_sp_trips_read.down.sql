-- Reverses TRACK-020 step 1: the trip reads.

DROP FUNCTION IF EXISTS tracking.sp_get_trip_status(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_trip(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_trips(UUID, DATE, UUID, VARCHAR, UUID, INT, INT);
DROP FUNCTION IF EXISTS tracking.fn_trip_stops_json(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_trip_rows(UUID, UUID[]);
