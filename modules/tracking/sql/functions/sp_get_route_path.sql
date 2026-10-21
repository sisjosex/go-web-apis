CREATE OR REPLACE FUNCTION tracking.sp_get_route_path(p_tenant_id uuid, p_route_id uuid, p_date date DEFAULT NULL::date, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(version_id uuid, polyline6 text, distance_m integer, legs jsonb, source character varying, computed_at timestamp with time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
    v_date  DATE;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;

    SELECT COALESCE(p_date, (now() AT TIME ZONE r.timezone)::date) INTO v_date
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
            AND (v_riders IS NULL OR rr.id = ANY(v_riders))
      ));

    IF v_date IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rv.id,
        rv.planned_polyline,
        rv.planned_distance_m,
        rv.planned_legs,
        CAST(rv.path_source AS VARCHAR),
        rv.path_computed_at
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id
      AND rv.effective_from <= v_date
      AND (rv.effective_to IS NULL OR rv.effective_to >= v_date);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_route_path(uuid,uuid,date,uuid) IS 'The planned line of the route version in force on p_date (the route''s local today when unset); raises route.not-found outside the tenant or p_scope_user_id''s organizations (TRACK-028)';
