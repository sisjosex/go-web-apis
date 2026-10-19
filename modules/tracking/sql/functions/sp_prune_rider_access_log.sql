CREATE OR REPLACE FUNCTION tracking.sp_prune_rider_access_log(p_tenant_id uuid, p_days integer)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted INT;
BEGIN
    DELETE FROM tracking.rider_access_log l
    WHERE l.tenant_id = p_tenant_id
      AND l.last_at < now() - p_days * interval '1 day';
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_prune_rider_access_log(uuid,integer) IS 'Drops the tenant''s access log rows last read more than p_days ago (TRACK-063)';
