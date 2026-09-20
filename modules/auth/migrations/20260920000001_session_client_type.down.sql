-- Reverse of TRACK-015 step 1: drop the four functions that carry p_client_type and let the earlier
-- migrations recreate them, then drop the column the constraint hangs off.

DROP FUNCTION IF EXISTS auth.sp_verify_otp(VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS auth.sp_login_email(VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS auth.sp_login_external(VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, VARCHAR);
DROP FUNCTION IF EXISTS auth.private_manage_user_session(UUID, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, TEXT, VARCHAR);

ALTER TABLE auth.user_sessions
    DROP CONSTRAINT IF EXISTS chk_user_sessions_client_type,
    DROP COLUMN IF EXISTS client_type;
