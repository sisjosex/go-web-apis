-- Reverses TRACK-022 steps 2 and 3: the absence and suggestion SPs and their helpers.

DROP FUNCTION IF EXISTS tracking.sp_suggest_rider_stops(UUID, UUID, VARCHAR, SMALLINT, INT, INT);
DROP FUNCTION IF EXISTS tracking.sp_delete_rider_absence(UUID, UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_create_rider_absence(UUID, UUID, DATE, DATE, VARCHAR, VARCHAR, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_absences(UUID, UUID, DATE, DATE, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_rider_absence_rows(UUID, UUID, UUID[], DATE, DATE);
DROP FUNCTION IF EXISTS tracking.fn_absence_routes_changed(UUID, DATE, DATE, VARCHAR);
DROP FUNCTION IF EXISTS tracking.fn_absence_first_pickup(UUID, UUID, DATE, DATE, VARCHAR);
DROP FUNCTION IF EXISTS tracking.fn_rider_absence_guard(UUID, UUID, UUID, UUID);
