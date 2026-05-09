CREATE OR REPLACE FUNCTION auth.sp_validate_reset_token(
    p_token TEXT
) RETURNS BOOLEAN AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM auth.password_reset_tokens
        WHERE token = p_token
          AND NOW() < expires_at
    ) THEN
        RAISE EXCEPTION 'reset-password.token-invalid'
            USING ERRCODE = 'P0001', DETAIL = 'Reset token is invalid, expired, or already used.';
    END IF;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
