CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_company_id uuid, p_is_active boolean)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        r.vehicle_id,
        r.route_name,
        r.route_code,
        r.origin_address,
        r.origin_lat,
        r.origin_lng,
        r.destination_address,
        r.destination_lat,
        r.destination_lng,
        r.schedule_type,
        r.scheduled_start_time,
        r.scheduled_end_time,
        r.estimated_duration_minutes,
        r.is_active,
        r.created_at,
        r.updated_at
    FROM tracking.routes r
    WHERE (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
    ORDER BY r.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_company_id uuid DEFAULT NULL::uuid, p_direction character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid, p_vehicle_id uuid DEFAULT NULL::uuid, p_organization_id uuid DEFAULT NULL::uuid, p_has_free_seats boolean DEFAULT NULL::boolean, p_sort character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, vehicle_id uuid, license_plate character varying, default_driver_id uuid, driver_name character varying, route_name character varying, route_code character varying, direction character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, estimated_duration_minutes integer, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, capacity integer, capacity_override integer, organization_id uuid, organization_name character varying, paired_route_id uuid, seats integer, assigned_count integer, total_count bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.company_id,
        CAST(tc.name AS VARCHAR),
        r.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        r.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(r.route_name AS VARCHAR),
        CAST(r.route_code AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        CAST(r.origin_address AS VARCHAR),
        r.origin_lat,
        r.origin_lng,
        CAST(r.destination_address AS VARCHAR),
        r.destination_lat,
        r.destination_lng,
        r.estimated_duration_minutes,
        CAST(r.timezone AS VARCHAR),
        r.is_active,
        r.created_at,
        r.updated_at,
        COALESCE(r.capacity, v.capacity),
        r.capacity,
        r.organization_id,
        CAST(o.name AS VARCHAR),
        r.paired_route_id,
        v.capacity,
        ac.n,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.routes r
    -- Not a LEFT JOIN: this join is what scopes the rows to the tenant.
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
    -- Riders the route carries today: the "32/40 seats" of its summary (TRACK-038 D3), and what the
    -- free-seat filter and order read (TRACK-046).
    CROSS JOIN LATERAL (
        SELECT CAST(COUNT(DISTINCT ra.rider_id) AS INT) AS n FROM tracking.rider_route_assignments ra
        WHERE ra.route_id = r.id
          AND ra.valid_from <= CURRENT_DATE
          AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
    ) ac
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR r.company_id = p_company_id)
      AND (p_vehicle_id IS NULL OR r.vehicle_id = p_vehicle_id)
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_direction IS NULL OR p_direction = '' OR r.direction = p_direction)
      AND (p_is_active IS NULL OR r.is_active = p_is_active)
      -- A route without seats (no vehicle) has none free.
      AND (NOT COALESCE(p_has_free_seats, false) OR ac.n < COALESCE(r.capacity, v.capacity))
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.route_name ILIKE '%' || p_search || '%'
          OR r.route_code ILIKE '%' || p_search || '%'
      )
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.rider_route_assignments ra
          INNER JOIN tracking.riders rr ON rr.id = ra.rider_id
          WHERE ra.route_id = r.id
            AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
            AND rr.organization_id = ANY(v_scope)
      ))
    -- Alphabetical, or most free seats first for "Asignar a ruta"; cut server-side, r.id breaks ties
    -- so two routes of one name never swap pages.
    ORDER BY
        CASE WHEN p_sort = 'free_seats' THEN COALESCE(r.capacity, v.capacity) - ac.n END DESC NULLS LAST,
        r.route_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_routes(uuid,character varying,uuid,character varying,boolean,integer,integer,uuid,uuid,uuid,boolean,character varying) IS 'One page of the tenant''s routes by name, narrowed by a name/code search, company, vehicle (TRACK-034), direction, active flag and free seats (TRACK-046), by name or by free seats; every row carries company name, plate, driver name and the total_count the filters match (TRACK-002 D2)';
