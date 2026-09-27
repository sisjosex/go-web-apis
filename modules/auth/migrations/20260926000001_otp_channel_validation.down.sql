-- Reverse of AUTH-001 step 1: drop the relay-aware sp_request_otp and sp_invalidate_otp, and put back
-- the three-argument sp_request_otp from 20260116170950_table_otp_requests.

DROP FUNCTION IF EXISTS auth.sp_invalidate_otp(UUID);
DROP FUNCTION IF EXISTS auth.sp_request_otp(VARCHAR, VARCHAR, VARCHAR, BOOLEAN);

CREATE FUNCTION auth.sp_request_otp(
    p_destination VARCHAR,          -- phone o email
    p_otp_channel VARCHAR,          -- 'whatsapp', 'sms', 'email'
    p_otp_code VARCHAR              -- 6-digit code generado por Go
)
RETURNS TABLE (
    otp_id UUID,
    destination VARCHAR,
    channel VARCHAR,
    expires_at TIMESTAMP WITH TIME ZONE,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_otp_id UUID;
    v_expires_at TIMESTAMP WITH TIME ZONE;
BEGIN
    -- Validación: Destination no puede estar vacío
    IF p_destination IS NULL OR TRIM(p_destination) = '' THEN
        RAISE EXCEPTION 'otp.destination.invalid' 
            USING ERRCODE = 'O0001', DETAIL = 'Destination (phone or email) is required';
    END IF;

    -- Validación: OTP Channel no puede estar vacío
    IF p_otp_channel IS NULL OR TRIM(p_otp_channel) = '' THEN
        RAISE EXCEPTION 'otp.channel.invalid' 
            USING ERRCODE = 'O0002', DETAIL = 'OTP channel is required';
    END IF;

    -- Validación: OTP Code debe ser 6 dígitos
    IF p_otp_code IS NULL OR p_otp_code !~ '^\d{6}$' THEN
        RAISE EXCEPTION 'otp.code.invalid' 
            USING ERRCODE = 'O0003', DETAIL = 'OTP code must be exactly 6 digits';
    END IF;

    -- Calcular fecha de expiración (configurado en Go, aquí solo almacenamos)
    v_expires_at := CURRENT_TIMESTAMP + interval '10 minutes';

    -- Insertar nuevo OTP
    INSERT INTO auth.otp_requests (
        destination, otp_channel, otp_code, expires_at, created_at, updated_at
    ) VALUES (
        TRIM(p_destination),
        LOWER(TRIM(p_otp_channel)),
        p_otp_code,
        v_expires_at,
        CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP
    )
    RETURNING id INTO v_otp_id;

    -- Retornar información
    RETURN QUERY
    SELECT 
        v_otp_id,
        CAST(TRIM(p_destination) AS VARCHAR),
        CAST(LOWER(TRIM(p_otp_channel)) AS VARCHAR),
        v_expires_at,
        'OTP code sent successfully to ' || CASE 
            WHEN LOWER(TRIM(p_otp_channel)) = 'email' THEN 'email'
            ELSE 'phone'
        END
    ;
END;
$$;
