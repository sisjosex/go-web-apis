CREATE OR REPLACE FUNCTION tracking.sp_get_route(p_route_id uuid)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE r.id = p_route_id;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_route(p_tenant_id uuid, p_route_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, vehicle_id uuid, license_plate character varying, default_driver_id uuid, driver_name character varying, route_name character varying, route_code character varying, direction character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, estimated_duration_minutes integer, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, capacity integer, capacity_override integer, organization_id uuid, organization_name character varying, paired_route_id uuid, seats integer, assigned_count integer)
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
        -- Riders the route carries today: the "32/40 seats" of its summary (TRACK-038 D3).
        CAST((
            SELECT COUNT(DISTINCT ra.rider_id) FROM tracking.rider_route_assignments ra
            WHERE ra.route_id = r.id
              AND ra.valid_from <= CURRENT_DATE
              AND (ra.valid_until IS NULL OR ra.valid_until >= CURRENT_DATE)
        ) AS INT)
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = r.vehicle_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.drivers d ON d.id = r.default_driver_id
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

    IF NOT FOUND THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_route(uuid,uuid,uuid) IS 'One route of the tenant with its company name, vehicle plate, default driver name, destination organization, paired route and today''s riders against its vehicle''s seats (TRACK-038); an organization user sees only routes their riders are assigned to; raises route.not-found (TRACK-002)';
