CREATE OR REPLACE FUNCTION tracking.sp_alert_client_users(p_tenant_id uuid, p_alert_id uuid)
 RETURNS TABLE(user_id uuid, alert_type character varying, delay_min integer, route_name character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT DISTINCT om.user_id, CAST(a.alert_type AS VARCHAR), a.estimated_delay_minutes, CAST(r.route_name AS VARCHAR)
    FROM tracking.route_alerts a
    INNER JOIN tracking.routes r ON r.id = a.route_id
    INNER JOIN tracking.trip_stops ts ON ts.trip_id = a.trip_id
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
    INNER JOIN tracking.riders rd ON rd.id = k.subject_id
    INNER JOIN tracking.organization_members om ON om.organization_id = rd.organization_id
    -- The join is the tenant guard, as fn_organization_scope's.
    INNER JOIN tracking.organizations o ON o.id = om.organization_id AND o.tenant_id = p_tenant_id
    WHERE a.id = p_alert_id
      AND a.tenant_id = p_tenant_id
      -- A driver's incident (stored with its kind as its title) that the driver let families and the client hear.
      AND a.notify_families
      AND a.title = a.alert_type
      AND k.status <> 'cancelled'
      -- TRACK-067 D2: a supervisor with groups hears only of the trips carrying its riders.
      AND (om.groups IS NULL OR rd.group_label = ANY(om.groups));
$function$;

COMMENT ON FUNCTION tracking.sp_alert_client_users(uuid,uuid) IS 'The accounts of the client (school or company) whose passengers ride the trip of a driver''s incident — every member of their organizations, a supervisor only for its groups — with the alert''s kind, minutes and route: who gets its push. None when the driver switched notify_families off (MOBILE-037 D2)';
