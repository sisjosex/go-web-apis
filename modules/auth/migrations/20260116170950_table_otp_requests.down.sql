-- Rollback: table_otp_requests
-- Module: auth
-- Description: Drop OTP-related stored procedures and table

DROP FUNCTION IF EXISTS auth.sp_verify_otp CASCADE;
DROP FUNCTION IF EXISTS auth.sp_request_otp CASCADE;
DROP TABLE IF EXISTS auth.otp_requests CASCADE;

