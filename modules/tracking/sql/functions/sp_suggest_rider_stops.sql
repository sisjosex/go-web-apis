CREATE OR REPLACE FUNCTION tracking.sp_suggest_rider_stops(p_tenant_id uuid, p_rider_id uuid, p_direction character varying, p_days smallint DEFAULT 127, p_max_walk_m integer DEFAULT 800, p_limit integer DEFAULT 10)
 RETURNS TABLE(route_id uuid, route_name character varying, direction character varying, stop_place_id uuid, stop_name character varying, latitude numeric, longitude numeric, walk_distance_m integer, capacity integer, assigned integer, free_seats integer)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_home GEOGRAPHY;
BEGIN
    SELECT r.home_location INTO v_home
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_home IS NULL THEN
        RAISE EXCEPTION 'rider.no-home-location' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH near AS (
        SELECT sp.id, sp.name, sp.location, ST_Distance(sp.location, v_home) AS distance
        FROM tracking.stop_places sp
        WHERE sp.tenant_id = p_tenant_id
          AND sp.location IS NOT NULL
          AND ST_DWithin(sp.location, v_home, p_max_walk_m)
    ),
    hits AS (
        SELECT DISTINCT ON (r.id, n.id)
            r.id AS route_id, r.route_name, r.direction, r.vehicle_id, r.timezone, r.capacity,
            n.id AS stop_place_id, n.name AS stop_name, n.location, n.distance
        FROM near n
        INNER JOIN tracking.route_version_stops vs ON vs.stop_place_id = n.id
        INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id AND tc.tenant_id = p_tenant_id
        CROSS JOIN LATERAL (SELECT CAST(now() AT TIME ZONE r.timezone AS DATE) AS today) lt
        WHERE r.is_active
          AND r.direction = p_direction
          AND rv.effective_from <= lt.today
          AND (rv.effective_to IS NULL OR rv.effective_to >= lt.today)
        ORDER BY r.id, n.id
    )
    SELECT
        h.route_id,
        CAST(h.route_name AS VARCHAR),
        CAST(h.direction AS VARCHAR),
        h.stop_place_id,
        CAST(h.stop_name AS VARCHAR),
        CAST(ST_Y(h.location::geometry) AS DECIMAL),
        CAST(ST_X(h.location::geometry) AS DECIMAL),
        CAST(ROUND(h.distance) AS INT),
        COALESCE(h.capacity, v.capacity),
        CAST(ld.assigned AS INT),
        COALESCE(h.capacity, v.capacity) - CAST(ld.assigned AS INT)
    FROM hits h
    LEFT JOIN tracking.vehicles v ON v.id = h.vehicle_id
    CROSS JOIN LATERAL (
        SELECT COUNT(*) AS assigned
        FROM tracking.rider_route_assignments a
        WHERE a.route_id = h.route_id
          AND a.valid_from <= CAST(now() AT TIME ZONE h.timezone AS DATE)
          AND (a.valid_until IS NULL OR a.valid_until >= CAST(now() AT TIME ZONE h.timezone AS DATE))
          AND (a.days_of_week & p_days) <> 0
    ) ld
    ORDER BY h.distance ASC, (COALESCE(h.capacity, v.capacity) - ld.assigned) DESC NULLS LAST, h.route_name, h.route_id
    LIMIT p_limit;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_suggest_rider_stops(uuid,uuid,character varying,smallint,integer,integer) IS 'The stops within p_max_walk_m of a rider''s home on the current version of an active route of p_direction, one per (route, stop), nearest first then most free seats; raises rider.not-found, rider.no-home-location (TRACK-022)';
