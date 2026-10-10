CREATE OR REPLACE FUNCTION auth.sp_invalidate_otp(p_otp_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE auth.otp_requests
    SET invalidated_at = CURRENT_TIMESTAMP,
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_otp_id
      AND invalidated_at IS NULL;
END;
$function$;
