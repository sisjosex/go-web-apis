-- Rollback: sp_invoice_operations
-- Module: purchasing

DROP FUNCTION IF EXISTS purchasing.sp_add_invoice(UUID, VARCHAR, DATE, DECIMAL, DECIMAL, DATE) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_mark_invoice_as_paid(UUID) CASCADE;
DROP FUNCTION IF EXISTS purchasing.sp_get_invoices(UUID) CASCADE;
