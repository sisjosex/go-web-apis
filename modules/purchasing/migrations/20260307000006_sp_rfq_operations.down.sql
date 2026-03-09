-- Rollback: sp_rfq_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_create_rfq(UUID, VARCHAR, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_item(UUID, UUID, DECIMAL, TEXT) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_add_rfq_response(UUID, UUID, DECIMAL, INT, VARCHAR, TEXT) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_items(UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_rfq_responses(UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_select_best_rfq_response(UUID, UUID) CASCADE;
