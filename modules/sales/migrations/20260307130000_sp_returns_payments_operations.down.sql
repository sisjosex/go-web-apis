-- Rollback: Returns & Payments Operations Stored Procedures

DROP FUNCTION IF EXISTS sales.sp_get_return(UUID);
DROP FUNCTION IF EXISTS sales.sp_get_returns_by_order(UUID);
DROP FUNCTION IF EXISTS sales.sp_get_payments(UUID);
DROP FUNCTION IF EXISTS sales.sp_get_payment_by_id(UUID);
