-- Reverse of TRACK-007 steps 2–4: the stop-place and route-version SPs go. The tables they read and
-- the outbox rows they wrote are 20260926000001's and the outbox migration's business, not this one's.

DROP FUNCTION IF EXISTS tracking.sp_create_stop_place(UUID, VARCHAR, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, UUID);
DROP FUNCTION IF EXISTS tracking.sp_update_stop_place(UUID, UUID, VARCHAR, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_stop_places(UUID, VARCHAR, DOUBLE PRECISION, DOUBLE PRECISION, INT, INT, INT);
DROP FUNCTION IF EXISTS tracking.sp_get_stop_place(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_delete_stop_place(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_replace_route_version_stops(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_create_route_version(UUID, UUID, DATE, JSONB, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_versions(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_route_version_stops_json(UUID, UUID, JSONB);
