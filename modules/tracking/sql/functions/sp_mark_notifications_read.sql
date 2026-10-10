CREATE OR REPLACE FUNCTION tracking.sp_mark_notifications_read(p_tenant_id uuid, p_user_id uuid, p_ids uuid[], p_all boolean DEFAULT false)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    UPDATE tracking.notifications n
    SET read_at = now()
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = n.rider_id
      AND o.tenant_id = p_tenant_id
      AND n.user_id = p_user_id
      AND n.read_at IS NULL
      AND (COALESCE(p_all, FALSE) OR n.id = ANY(COALESCE(p_ids, '{}')));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_mark_notifications_read(uuid,uuid,uuid[],boolean) IS 'Stamps read_at on the account''s unread rows in p_ids, or on all of them when p_all; others'' ids are ignored (TRACK-012)';
