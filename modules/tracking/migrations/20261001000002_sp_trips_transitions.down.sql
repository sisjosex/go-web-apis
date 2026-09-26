-- Reverses TRACK-020 step 2: the trip writes. trip_events and outbox rows they left stay as history.

DROP FUNCTION IF EXISTS tracking.sp_update_trip(UUID, UUID, JSONB, UUID);
DROP FUNCTION IF EXISTS tracking.sp_trip_task_transition(UUID, UUID, VARCHAR, UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_trip_stop_transition(UUID, UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_trip_transition(UUID, UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.fn_trip_lock(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_trip_changed(UUID, UUID, VARCHAR, UUID, JSONB);
