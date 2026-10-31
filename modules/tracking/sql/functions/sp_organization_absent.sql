CREATE OR REPLACE FUNCTION tracking.sp_organization_absent(p_tenant_id uuid, p_scope_user_id uuid, p_date date DEFAULT NULL::date, p_organization_id uuid DEFAULT NULL::uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope  UUID[];
    v_riders UUID[];
    v_org    tracking.organizations%ROWTYPE;
    v_date   DATE;
    v_rows   JSONB;
BEGIN
    -- The same scope as the client's day (sp_organization_today): its own organizations, or the one staff names.
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    ELSIF p_organization_id IS NULL THEN
        RAISE EXCEPTION 'organization.required' USING ERRCODE = 'P0001';
    END IF;

    SELECT o.* INTO v_org
    FROM tracking.organizations o
    WHERE o.tenant_id = p_tenant_id
      AND (p_organization_id IS NULL OR o.id = p_organization_id)
      AND (v_scope IS NULL OR o.id = ANY(v_scope))
    ORDER BY o.name, o.id
    LIMIT 1;
    IF v_org.id IS NULL THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    v_date := COALESCE(p_date, CAST(now() AT TIME ZONE v_org.timezone AS DATE));

    -- The riders the day's total counts as absent: a cancelled task on a trip of that date.
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
               'rider_id', a.rider_id,
               'name', a.name,
               'trips', a.trips
           ) ORDER BY a.name, a.rider_id), '[]'::jsonb)
    INTO v_rows
    FROM (
        SELECT rd.id AS rider_id,
               TRIM(rd.first_name || ' ' || COALESCE(rd.last_name, '')) AS name,
               COUNT(DISTINCT tr.id) AS trips
        FROM tracking.trips tr
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = tr.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
        INNER JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE tr.tenant_id = p_tenant_id
          AND tr.service_date = v_date
          AND tr.status <> 'cancelled'
          AND k.status = 'cancelled'
          AND rd.organization_id = v_org.id
          AND (v_riders IS NULL OR rd.id = ANY(v_riders))
        GROUP BY rd.id, rd.first_name, rd.last_name
    ) a;

    RETURN jsonb_build_object('riders', v_rows);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_organization_absent(uuid,uuid,date,uuid) IS 'Who the client''s day counts as absent, by name: the row under its totals opens this list (MOBILE-041).';
