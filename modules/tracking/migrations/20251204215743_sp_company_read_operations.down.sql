-- Rollback: sp_company_read_operations
-- Module: tracking

DROP FUNCTION IF EXISTS tracking.sp_list_companies CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_get_company CASCADE;
DROP FUNCTION IF EXISTS tracking.sp_delete_company CASCADE;
