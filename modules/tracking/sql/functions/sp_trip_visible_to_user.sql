CREATE OR REPLACE FUNCTION tracking.sp_trip_visible_to_user(p_tenant_id uuid, p_trip_id uuid, p_user_id uuid)
 RETURNS boolean
 LANGUAGE sql
 STABLE
AS $function$
    SELECT EXISTS (
        SELECT 1
        FROM tracking.trips t
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
        INNER JOIN tracking.riders rd ON rd.id = k.subject_id
        INNER JOIN tracking.organization_members om ON om.organization_id = rd.organization_id
        WHERE t.id = p_trip_id
          AND t.tenant_id = p_tenant_id
          AND om.user_id = p_user_id);
$function$;

COMMENT ON FUNCTION tracking.sp_trip_visible_to_user(uuid,uuid,uuid) IS 'Whether an organization user may watch a trip live: it carries a passenger of one of their organizations (MOBILE-020)';
