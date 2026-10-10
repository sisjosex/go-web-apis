CREATE OR REPLACE FUNCTION tracking.sp_get_notification_settings(p_tenant_id uuid, p_user_id uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, muted character varying[])
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT r.id,
           CAST(r.first_name || ' ' || r.last_name AS VARCHAR),
           COALESCE(
               (SELECT ARRAY_AGG(CAST(s.type AS VARCHAR) ORDER BY s.type)
                FROM tracking.notification_settings s
                WHERE s.user_id = p_user_id AND s.rider_id = r.id),
               ARRAY[]::VARCHAR[])
    FROM tracking.riders r
    WHERE r.id = ANY(tracking.fn_notice_scope(p_tenant_id, p_user_id))
    ORDER BY r.first_name, r.last_name, r.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_notification_settings(uuid,uuid) IS 'Each rider an account receives notices for, with the types it muted (TRACK-012)';
