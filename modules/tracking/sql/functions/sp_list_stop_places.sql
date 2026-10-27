CREATE OR REPLACE FUNCTION tracking.sp_list_stop_places(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_latitude double precision DEFAULT NULL::double precision, p_longitude double precision DEFAULT NULL::double precision, p_radius_m integer DEFAULT NULL::integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_organization_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, address character varying, latitude numeric, longitude numeric, distance_m numeric, created_at timestamp without time zone, updated_at timestamp without time zone, riders_count integer, routes_count integer, route_name character varying, is_destination boolean, total_count bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_point  GEOGRAPHY;
    v_radius INT := COALESCE(p_radius_m, 1000);
BEGIN
    IF p_latitude IS NOT NULL AND p_longitude IS NOT NULL THEN
        v_point := ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography;
    END IF;

    RETURN QUERY
    WITH pg AS (
        SELECT
            sp.id,
            CASE WHEN v_point IS NULL THEN NULL ELSE CAST(ST_Distance(sp.location, v_point) AS DECIMAL) END AS distance_m,
            CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM tracking.stop_places sp
        WHERE sp.tenant_id = p_tenant_id
          AND (p_search IS NULL OR p_search = '' OR sp.name ILIKE '%' || p_search || '%')
          -- TRACK-070 D5: a client's gates are the places that belong to it.
          AND (p_organization_id IS NULL OR sp.organization_id = p_organization_id)
          AND (v_point IS NULL OR (sp.location IS NOT NULL AND ST_DWithin(sp.location, v_point, v_radius)))
        -- Nearest first when a point was given; alphabetical otherwise, with sp.id breaking ties so the
        -- page boundary follows the order the reader sees.
        ORDER BY
            CASE WHEN v_point IS NULL THEN NULL ELSE ST_Distance(sp.location, v_point) END ASC NULLS LAST,
            sp.name ASC,
            sp.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        pg.distance_m,
        sp.created_at,
        sp.updated_at,
        COALESCE(rc.n, 0),
        COALESCE(u.n, 0),
        CAST(u.first_name AS VARCHAR),
        (COALESCE(u.is_last, false) OR EXISTS (
            SELECT 1 FROM tracking.organizations o WHERE o.tenant_id = p_tenant_id AND o.stop_place_id = sp.id
        )),
        pg.total_count
    FROM pg
    INNER JOIN tracking.stop_places sp ON sp.id = pg.id
    -- TRACK-075, for the page only: the routes whose stop list in force on their own today has the
    -- place, the first by name, and whether it ends one of them.
    LEFT JOIN LATERAL (
        SELECT
            CAST(COUNT(DISTINCT r.id) AS INT) AS n,
            (array_agg(r.route_name ORDER BY r.route_name, r.id))[1] AS first_name,
            bool_or(vs.sequence = (SELECT MAX(s2.sequence) FROM tracking.route_version_stops s2 WHERE s2.version_id = rv.id)) AS is_last,
            array_agg(DISTINCT r.id) AS route_ids
        FROM tracking.route_version_stops vs
        INNER JOIN tracking.route_versions rv ON rv.id = vs.version_id
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        WHERE vs.stop_place_id = sp.id
          AND rv.effective_from <= CAST(now() AT TIME ZONE r.timezone AS DATE)
          AND (rv.effective_to IS NULL OR rv.effective_to >= CAST(now() AT TIME ZONE r.timezone AS DATE))
    ) u ON true
    -- The riders whose assignment in force today boards or leaves at it, read through those routes
    -- so the probe stays on the route index.
    LEFT JOIN LATERAL (
        SELECT CAST(COUNT(DISTINCT ra.rider_id) AS INT) AS n
        FROM tracking.rider_route_assignments ra
        WHERE ra.route_id = ANY(u.route_ids)
          AND (ra.pickup_stop_place_id = sp.id OR ra.dropoff_stop_place_id = sp.id)
          AND ra.valid_from <= CURRENT_DATE
          AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
    ) rc ON true
    ORDER BY
        pg.distance_m ASC NULLS LAST,
        sp.name ASC,
        sp.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_stop_places(uuid,character varying,double precision,double precision,integer,integer,integer,uuid) IS 'One page of a tenant''s stop places, or of one client''s (TRACK-070 D5), by name or — when a point is given — within p_radius_m of it, nearest first and carrying distance_m; every row carries the riders boarding or leaving there today, the routes whose stop list has it and the first by name, whether it is a destination (a client''s stop or a route''s last — TRACK-075) and the total_count the filters match';
