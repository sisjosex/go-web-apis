CREATE OR REPLACE FUNCTION tracking.sp_list_stop_places(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_latitude double precision DEFAULT NULL::double precision, p_longitude double precision DEFAULT NULL::double precision, p_radius_m integer DEFAULT NULL::integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_organization_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, organization_id uuid, name character varying, address character varying, latitude numeric, longitude numeric, distance_m numeric, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
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
    SELECT
        sp.id,
        sp.tenant_id,
        sp.organization_id,
        CAST(sp.name AS VARCHAR),
        CAST(sp.address AS VARCHAR),
        CAST(ST_Y(sp.location::geometry) AS DECIMAL),
        CAST(ST_X(sp.location::geometry) AS DECIMAL),
        CASE WHEN v_point IS NULL THEN NULL ELSE CAST(ST_Distance(sp.location, v_point) AS DECIMAL) END,
        sp.created_at,
        sp.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
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
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_stop_places(uuid,character varying,double precision,double precision,integer,integer,integer,uuid) IS 'One page of a tenant''s stop places, or of one client''s (TRACK-070 D5), by name or — when a point is given — within p_radius_m of it, nearest first and carrying distance_m; every row carries the total_count the filters match';
