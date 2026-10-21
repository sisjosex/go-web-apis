CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(p_route_id uuid)
 RETURNS TABLE(id uuid, route_id uuid, stop_name character varying, address character varying, latitude numeric, longitude numeric, stop_order integer, scheduled_arrival_offset_minutes integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        rs.id,
        rs.route_id,
        rs.location_name AS stop_name,
        ''::VARCHAR(500) AS address, -- TODO: add address field to table if needed
        rs.latitude,
        rs.longitude,
        rs.stop_order,
        EXTRACT(HOUR FROM rs.estimated_arrival)::INT * 60 + EXTRACT(MINUTE FROM rs.estimated_arrival)::INT AS scheduled_arrival_offset_minutes,
        rs.created_at,
        rs.updated_at
    FROM tracking.route_stops rs
    WHERE rs.route_id = p_route_id
      AND rs.status = 'active'
    ORDER BY rs.stop_order ASC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_date date DEFAULT NULL::date)
 RETURNS TABLE(id uuid, route_id uuid, version_id uuid, stop_place_id uuid, stop_name character varying, address character varying, latitude numeric, longitude numeric, sequence integer, planned_offset_min integer, dwell_sec integer, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
    v_date DATE := COALESCE(p_date, CURRENT_DATE);
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);

        IF NOT EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
            WHERE r.id = p_route_id
              AND tc.tenant_id = p_tenant_id
              AND EXISTS (
                  SELECT 1
                  FROM tracking.rider_route_assignments ra
                  INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
                  WHERE ra.route_id = r.id
                    AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
                    AND rr.organization_id = ANY(v_scope)
                    AND (v_riders IS NULL OR rr.id = ANY(v_riders))
              )
        ) THEN
            RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    SELECT
        vs.id,
        rv.route_id,
        rv.id,
        sp.id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        vs.sequence,
        vs.planned_offset_min,
        vs.dwell_sec,
        vs.created_at,
        vs.updated_at
    FROM tracking.route_versions rv
    INNER JOIN tracking.routes r ON r.id = rv.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    INNER JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
    INNER JOIN tracking.stop_places sp ON sp.id = vs.stop_place_id
    WHERE rv.route_id = p_route_id
      AND tc.tenant_id = p_tenant_id
      AND rv.effective_from <= v_date
      AND (rv.effective_to IS NULL OR rv.effective_to >= v_date)
    ORDER BY vs.sequence ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_stops(uuid,uuid,uuid,date) IS 'The stops a route runs on p_date (today when unset), read from the version in force that day (TRACK-007 D1); a route outside p_scope_user_id''s organizations raises route.not-found';
