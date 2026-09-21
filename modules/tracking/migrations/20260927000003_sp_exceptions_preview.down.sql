-- Reverses 20260927000003. The payload check goes last: the create SP calls it.

DROP FUNCTION IF EXISTS tracking.sp_preview_route(UUID, UUID, DATE, DATE);
DROP FUNCTION IF EXISTS tracking.sp_delete_route_exception(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_create_route_exception(UUID, UUID, DATE, DATE, VARCHAR, JSONB, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_route_exceptions(UUID, UUID, DATE, DATE);
DROP FUNCTION IF EXISTS tracking.fn_exception_payload_valid(VARCHAR, JSONB);
