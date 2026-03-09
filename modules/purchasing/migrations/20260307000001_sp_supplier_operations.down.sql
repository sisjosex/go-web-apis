-- Rollback: sp_supplier_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_create_supplier(UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, INT) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_update_supplier(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, INT, BOOLEAN) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_delete_supplier(UUID, UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_suppliers_count(UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_list_suppliers(UUID, INT, INT) CASCADE;
