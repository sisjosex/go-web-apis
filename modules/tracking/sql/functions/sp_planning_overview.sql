CREATE OR REPLACE FUNCTION tracking.sp_planning_overview(p_tenant_id uuid, p_organization_id uuid, p_days smallint DEFAULT 31)
 RETURNS TABLE(rider_id uuid, rider_name character varying, lat numeric, lng numeric, service_legs character varying, outbound_route_id uuid, outbound_route_name character varying, return_route_id uuid, return_route_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rd.id,
        CAST(TRIM(rd.first_name || ' ' || rd.last_name) AS VARCHAR),
        CAST(ST_Y(pt.location::geometry) AS DECIMAL),
        CAST(ST_X(pt.location::geometry) AS DECIMAL),
        CAST(rd.service_legs AS VARCHAR),
        ob.route_id,
        ob.route_name,
        rt.route_id,
        rt.route_name
    FROM tracking.riders rd
    LEFT JOIN tracking.stop_places sp ON sp.id = rd.stop_place_id AND sp.location IS NOT NULL
    CROSS JOIN LATERAL (SELECT COALESCE(sp.location, rd.home_location) AS location) pt
    LEFT JOIN LATERAL (
        SELECT a.route_id, CAST(r.route_name AS VARCHAR) AS route_name
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes r ON r.id = a.route_id
        WHERE a.rider_id = rd.id
          AND r.direction = 'outbound'
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (a.days_of_week & p_days) <> 0
        ORDER BY a.valid_from
        LIMIT 1
    ) ob ON true
    LEFT JOIN LATERAL (
        SELECT a.route_id, CAST(r.route_name AS VARCHAR) AS route_name
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes r ON r.id = a.route_id
        WHERE a.rider_id = rd.id
          AND r.direction = 'inbound'
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (a.days_of_week & p_days) <> 0
        ORDER BY a.valid_from
        LIMIT 1
    ) rt ON true
    WHERE rd.organization_id = p_organization_id
      AND COALESCE(rd.status, 'active') = 'active'
    ORDER BY rd.last_name, rd.first_name, rd.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_planning_overview(uuid,uuid,smallint) IS 'The organization''s active riders for its planning screen (TRACK-014 step 1): pickup point (shared stop, else home; NULL: no pin), contracted legs and the outbound and return route each already rides on one of p_days; raises organization.not-found';
