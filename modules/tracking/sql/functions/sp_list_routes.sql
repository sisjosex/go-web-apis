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

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_company_id uuid DEFAULT NULL::uuid, p_direction character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid, p_vehicle_id uuid DEFAULT NULL::uuid, p_organization_id uuid DEFAULT NULL::uuid, p_has_free_seats boolean DEFAULT NULL::boolean, p_sort character varying DEFAULT NULL::character varying, p_group_pairs boolean DEFAULT false, p_without_vehicle boolean DEFAULT false, p_without_driver boolean DEFAULT false)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, vehicle_id uuid, license_plate character varying, default_driver_id uuid, driver_name character varying, route_name character varying, route_code character varying, direction character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, estimated_duration_minutes integer, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, capacity integer, capacity_override integer, organization_id uuid, organization_name character varying, paired_route_id uuid, seats integer, assigned_count integer, start_time character varying, arrival_time character varying, organization_kind character varying, return_route jsonb, counts jsonb, total_count bigint)
 LANGUAGE plpgsql
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
    -- b: every filter but the three the chips toggle (direction, without driver, free seats), so
    -- the chips' counts read it (TRACK-075).
    WITH b AS MATERIALIZED (
        SELECT
            r.id,
            r.company_id,
            CAST(tc.name AS VARCHAR) AS company_name,
            r.vehicle_id,
            CAST(v.plate_number AS VARCHAR) AS license_plate,
            r.default_driver_id,
            CAST(d.first_name || ' ' || d.last_name AS VARCHAR) AS driver_name,
            CAST(r.route_name AS VARCHAR) AS route_name,
            CAST(r.route_code AS VARCHAR) AS route_code,
            CAST(r.direction AS VARCHAR) AS direction,
            CAST(r.origin_address AS VARCHAR) AS origin_address,
            r.origin_lat,
            r.origin_lng,
            CAST(r.destination_address AS VARCHAR) AS destination_address,
            r.destination_lat,
            r.destination_lng,
            r.estimated_duration_minutes,
            CAST(r.timezone AS VARCHAR) AS timezone,
            r.is_active,
            r.created_at,
            r.updated_at,
            COALESCE(r.capacity, v.capacity) AS capacity,
            r.capacity AS capacity_override,
            r.organization_id,
            CAST(o.name AS VARCHAR) AS organization_name,
            r.paired_route_id,
            v.capacity AS seats,
            ac.n AS assigned_count,
            -- TRACK-070 D1: the schedule in force on the route's today, its departure and the arrival the estimate
            -- keeps (TRACK-055 D2); one indexed read per route.
            CAST(TO_CHAR(sc.start_time, 'HH24:MI') AS VARCHAR) AS start_time,
            CAST(TO_CHAR(sc.start_time + make_interval(mins => sc.departure_auto_min), 'HH24:MI') AS VARCHAR) AS arrival_time,
            CAST(o.kind AS VARCHAR) AS organization_kind,
            -- A route's trips take the vehicle's usual driver when it has none (sp_preview_route), so
            -- "without driver" is neither.
            (r.default_driver_id IS NULL AND v.default_driver_id IS NULL) AS no_driver,
            -- A route without seats (no vehicle) has none free.
            COALESCE(ac.n < COALESCE(r.capacity, v.capacity), false) AS free_seat
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
        -- In force on the route's own today, in its zone: a schedule made today starts there.
        LEFT JOIN LATERAL (
            SELECT s.start_time, s.departure_auto_min FROM tracking.route_schedules s
            WHERE s.route_id = r.id
              AND s.valid_from <= CAST(now() AT TIME ZONE r.timezone AS DATE)
              AND (s.valid_until IS NULL OR s.valid_until >= CAST(now() AT TIME ZONE r.timezone AS DATE))
            ORDER BY s.valid_from DESC, s.start_time ASC
            LIMIT 1
        ) sc ON true
        WHERE tc.tenant_id = p_tenant_id
          AND (p_company_id IS NULL OR r.company_id = p_company_id)
          AND (p_vehicle_id IS NULL OR r.vehicle_id = p_vehicle_id)
          AND (NOT COALESCE(p_without_vehicle, false) OR r.vehicle_id IS NULL)
          AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
          AND (p_is_active IS NULL OR r.is_active = p_is_active)
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
                AND (v_riders IS NULL OR rr.id = ANY(v_riders))
          ))
    ),
    f AS MATERIALIZED (
        SELECT * FROM b
        WHERE (p_direction IS NULL OR p_direction = '' OR b.direction = p_direction)
          AND (NOT COALESCE(p_without_driver, false) OR b.no_driver)
          AND (NOT COALESCE(p_has_free_seats, false) OR b.free_seat)
    ),
    -- "all" counts what the unfiltered list shows: grouped, a pair is one service; the rest count routes.
    c AS (
        SELECT jsonb_build_object(
            'all', COUNT(*) FILTER (WHERE NOT (COALESCE(p_group_pairs, false) AND b.direction = 'inbound'
                     AND EXISTS (SELECT 1 FROM b ob WHERE ob.id = b.paired_route_id AND ob.direction = 'outbound'))),
            'outbound',       COUNT(*) FILTER (WHERE b.direction = 'outbound'),
            'inbound',        COUNT(*) FILTER (WHERE b.direction = 'inbound'),
            'without_driver', COUNT(*) FILTER (WHERE b.no_driver),
            'free_seats',     COUNT(*) FILTER (WHERE b.free_seat)
        ) AS counts
        FROM b
    ),
    pg AS (
        SELECT
            f.id, f.company_id, f.company_name, f.vehicle_id, f.license_plate, f.default_driver_id, f.driver_name,
            f.route_name, f.route_code, f.direction, f.origin_address, f.origin_lat, f.origin_lng,
            f.destination_address, f.destination_lat, f.destination_lng, f.estimated_duration_minutes, f.timezone,
            f.is_active, f.created_at, f.updated_at, f.capacity, f.capacity_override, f.organization_id,
            f.organization_name, f.paired_route_id, f.seats, f.assigned_count, f.start_time, f.arrival_time,
            f.organization_kind,
            CASE WHEN pr.id IS NOT NULL THEN to_jsonb(pr) - ARRAY['no_driver', 'free_seat'] END AS return_route,
            CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM f
        -- TRACK-070 D1: grouped, a return whose outbound the same filters keep is that outbound's
        -- return_route, not a row, so the count is services; a return whose outbound is filtered out
        -- lists alone.
        LEFT JOIN f pr ON COALESCE(p_group_pairs, false) AND f.direction = 'outbound' AND pr.id = f.paired_route_id
        WHERE NOT (COALESCE(p_group_pairs, false) AND f.direction = 'inbound'
                   AND EXISTS (SELECT 1 FROM f ob WHERE ob.id = f.paired_route_id AND ob.direction = 'outbound'))
        -- Alphabetical, or most free seats first for "Asignar a ruta"; cut server-side, f.id breaks ties
        -- so two routes of one name never swap pages.
        ORDER BY
            CASE WHEN p_sort = 'free_seats' THEN f.capacity - f.assigned_count END DESC NULLS LAST,
            f.route_name ASC, f.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        pg.id, pg.company_id, pg.company_name, pg.vehicle_id, pg.license_plate, pg.default_driver_id, pg.driver_name,
        pg.route_name, pg.route_code, pg.direction, pg.origin_address, pg.origin_lat, pg.origin_lng,
        pg.destination_address, pg.destination_lat, pg.destination_lng, pg.estimated_duration_minutes, pg.timezone,
        pg.is_active, pg.created_at, pg.updated_at, pg.capacity, pg.capacity_override, pg.organization_id,
        pg.organization_name, pg.paired_route_id, pg.seats, pg.assigned_count, pg.start_time, pg.arrival_time,
        pg.organization_kind, pg.return_route, c.counts, COALESCE(pg.total_count, 0)
    -- An empty page still answers one row, id NULL, so the chips keep their counts.
    FROM c
    LEFT JOIN pg ON true
    ORDER BY
        CASE WHEN p_sort = 'free_seats' THEN pg.capacity - pg.assigned_count END DESC NULLS LAST,
        pg.route_name ASC, pg.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_routes(uuid,character varying,uuid,character varying,boolean,integer,integer,uuid,uuid,uuid,boolean,character varying,boolean,boolean,boolean) IS 'One page of the tenant''s routes by name, narrowed by a name/code search, company, vehicle or no vehicle (TRACK-034, TRACK-070 D4), direction, active flag, free seats (TRACK-046) and no driver on the route nor its vehicle (TRACK-075), by name or by free seats; every row carries company name, plate, driver name, the client''s kind, the schedule in force today, the chip counts over every filter but direction, driver and free seats, and the total_count the filters match (TRACK-002 D2); with p_group_pairs a pair is one row, its return as return_route (TRACK-070 D1); an empty page is one row with id NULL that still carries the counts';
