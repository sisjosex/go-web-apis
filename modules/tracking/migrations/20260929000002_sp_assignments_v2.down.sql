-- Reverses TRACK-009 step 2: the assignment endpoints' SPs. The table and the readers are step 1's.

DROP FUNCTION IF EXISTS tracking.sp_delete_rider_route_assignment(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_update_rider_route_assignment(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_create_rider_route_assignments(UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_route_assignments(UUID, UUID, UUID, DATE, UUID, INT, INT);
DROP FUNCTION IF EXISTS tracking.fn_assignment_warnings(UUID, UUID[]);
DROP FUNCTION IF EXISTS tracking.fn_assignment_put(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.fn_rider_route_assignment_rows(UUID, UUID[]);
