-- Reverses 20260927000002. The helpers go last: the schedule SPs call them.

DROP FUNCTION IF EXISTS tracking.sp_split_route_schedule(UUID, UUID, DATE, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_delete_route_schedule(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_update_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID);
DROP FUNCTION IF EXISTS tracking.sp_create_route_schedule(UUID, UUID, SMALLINT, TIME, DATE, DATE, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_schedules(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_route_schedule_guard(UUID, UUID, TIME, DATE, DATE, UUID, UUID);

DROP FUNCTION IF EXISTS tracking.sp_replace_calendar_dates(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_list_calendar_dates(UUID, UUID, DATE, DATE);
DROP FUNCTION IF EXISTS tracking.sp_delete_calendar(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_update_calendar(UUID, UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_create_calendar(UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_calendars(UUID, VARCHAR, INT, INT);

DROP FUNCTION IF EXISTS tracking.fn_route_schedule_row(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_route_changed(UUID, DATE, DATE);
