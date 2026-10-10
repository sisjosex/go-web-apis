CREATE OR REPLACE FUNCTION auth.sp_request_otp(p_destination character varying, p_otp_channel character varying, p_otp_code character varying, p_relay_to_email boolean DEFAULT false)
 RETURNS TABLE(otp_id uuid, destination character varying, channel character varying, expires_at timestamp with time zone, relay_email character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_destination VARCHAR := TRIM(p_destination);
    v_channel VARCHAR := LOWER(TRIM(p_otp_channel));
    v_relay_email VARCHAR;
    v_otp_id UUID;
    v_expires_at TIMESTAMP WITH TIME ZONE := CURRENT_TIMESTAMP + interval '10 minutes';
BEGIN
    IF v_destination IS NULL OR v_destination = '' THEN
        RAISE EXCEPTION 'otp.destination.invalid'
            USING ERRCODE = 'O0001', DETAIL = 'Destination (phone or email) is required';
    END IF;

    IF v_channel IS NULL OR v_channel NOT IN ('whatsapp', 'sms', 'email') THEN
        RAISE EXCEPTION 'otp.channel.invalid'
            USING ERRCODE = 'O0002', DETAIL = 'OTP channel must be whatsapp, sms or email';
    END IF;

    IF p_otp_code IS NULL OR p_otp_code !~ '^\d{6}$' THEN
        RAISE EXCEPTION 'otp.code.invalid'
            USING ERRCODE = 'O0003', DETAIL = 'OTP code must be exactly 6 digits';
    END IF;

    IF p_relay_to_email THEN
        IF v_channel = 'email' THEN
            RAISE EXCEPTION 'otp.channel.invalid'
                USING ERRCODE = 'O0002', DETAIL = 'Only a phone channel can be relayed by email';
        END IF;

        SELECT u.email INTO v_relay_email
        FROM auth.users u
        WHERE u.phone = v_destination
          AND u.is_active = TRUE
          AND u.email IS NOT NULL
        LIMIT 1;

        IF v_relay_email IS NULL THEN
            RAISE EXCEPTION 'otp.phone.not-registered'
                USING ERRCODE = 'O0007', DETAIL = 'No account holds this phone; sign in with email';
        END IF;
    END IF;

    INSERT INTO auth.otp_requests (destination, otp_channel, otp_code, expires_at)
    VALUES (v_destination, v_channel, p_otp_code, v_expires_at)
    RETURNING id INTO v_otp_id;

    RETURN QUERY SELECT v_otp_id, v_destination, v_channel, v_expires_at, v_relay_email;
END;
$function$;
