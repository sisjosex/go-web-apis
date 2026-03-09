-- Rollback: sp_product_batch_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_create_product_batch(UUID, UUID, VARCHAR, DECIMAL, DECIMAL, DATE, DATE, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_product_batches(UUID, UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_oldest_batch_for_sale(UUID, UUID) CASCADE;
