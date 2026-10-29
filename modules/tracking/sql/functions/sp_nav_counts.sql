CREATE OR REPLACE FUNCTION tracking.sp_nav_counts(p_tenant_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(trips_in_progress integer, alerts_active integer, documents_expiring integer)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        -- Today in each trip's own time zone, as sp_tracking_overview counts it.
        CAST((
            SELECT COUNT(*)
            FROM tracking.trips t
            WHERE t.tenant_id = p_tenant_id
              AND t.status = 'in_progress'
              AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
              AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
              AND (v_scope IS NULL OR EXISTS (
                  SELECT 1
                  FROM tracking.trip_stops ts
                  INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
                  INNER JOIN tracking.riders rd ON rd.id = k.subject_id
                  WHERE ts.trip_id = t.id AND k.subject_type = 'passenger' AND rd.organization_id = ANY(v_scope)
                    AND (v_riders IS NULL OR rd.id = ANY(v_riders))
              ))
        ) AS INT),
        -- An organization user sees neither alerts nor documents (TRACK-067).
        CASE WHEN p_scope_user_id IS NULL THEN CAST((
            SELECT COUNT(*)
            FROM tracking.route_alerts a
            WHERE a.tenant_id = p_tenant_id AND a.status = 'active'
        ) AS INT) END,
        -- The same window as sp_list_documents' p_expiring_within_days = 30: expired ones included.
        CASE WHEN p_scope_user_id IS NULL THEN CAST((
            SELECT COUNT(*)
            FROM tracking.compliance_documents d
            WHERE d.tenant_id = p_tenant_id
              AND d.expires_on IS NOT NULL
              AND d.expires_on <= CURRENT_DATE + 30
        ) AS INT) END;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_nav_counts(uuid,uuid) IS 'The taypi go rail''s badges in one read (TRACK-081): today''s trips in progress, active alerts and documents expired or due within 30 days; narrowed to p_scope_user_id''s organizations when set, which leaves alerts and documents NULL';
