CREATE OR REPLACE FUNCTION tracking.sp_create_route(p_company_id uuid, p_route_name character varying, p_origin_address character varying, p_destination_address character varying, p_route_code character varying, p_vehicle_id uuid, p_origin_lat numeric, p_origin_lng numeric, p_destination_lat numeric, p_destination_lng numeric, p_schedule_type character varying, p_scheduled_start_time time without time zone, p_scheduled_end_time time without time zone, p_estimated_duration_minutes integer)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies tc WHERE tc.id = p_company_id) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if vehicle exists (if provided)
    IF p_vehicle_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
            RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Check schedule_type is valid
    IF p_schedule_type NOT IN ('morning', 'afternoon', 'custom') THEN
        RAISE EXCEPTION 'route.invalid-schedule-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if route_code is unique for this company (if provided)
    IF p_route_code IS NOT NULL THEN
        IF EXISTS (SELECT 1 FROM tracking.routes r WHERE r.company_id = p_company_id AND r.route_code = p_route_code) THEN
            RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    INSERT INTO tracking.routes (
        company_id, route_name, origin_address, destination_address, route_code, vehicle_id,
        origin_lat, origin_lng, destination_lat, destination_lng, schedule_type,
        scheduled_start_time, scheduled_end_time, estimated_duration_minutes, is_active
    )
    VALUES (
        p_company_id, p_route_name, p_origin_address, p_destination_address, p_route_code, p_vehicle_id,
        p_origin_lat, p_origin_lng, p_destination_lat, p_destination_lng, p_schedule_type,
        p_scheduled_start_time, p_scheduled_end_time, p_estimated_duration_minutes, true
    )
    RETURNING
        tracking.routes.id,
        tracking.routes.company_id,
        tracking.routes.vehicle_id,
        tracking.routes.route_name,
        tracking.routes.route_code,
        tracking.routes.origin_address,
        tracking.routes.origin_lat,
        tracking.routes.origin_lng,
        tracking.routes.destination_address,
        tracking.routes.destination_lat,
        tracking.routes.destination_lng,
        tracking.routes.schedule_type,
        tracking.routes.scheduled_start_time,
        tracking.routes.scheduled_end_time,
        tracking.routes.estimated_duration_minutes,
        tracking.routes.is_active,
        tracking.routes.created_at,
        tracking.routes.updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_create_route(p_tenant_id uuid, p_company_id uuid, p_route_name character varying, p_origin_address character varying, p_destination_address character varying, p_direction character varying DEFAULT NULL::character varying, p_vehicle_id uuid DEFAULT NULL::uuid, p_default_driver_id uuid DEFAULT NULL::uuid, p_timezone character varying DEFAULT NULL::character varying, p_route_code character varying DEFAULT NULL::character varying, p_origin_lat numeric DEFAULT NULL::numeric, p_origin_lng numeric DEFAULT NULL::numeric, p_destination_lat numeric DEFAULT NULL::numeric, p_destination_lng numeric DEFAULT NULL::numeric, p_estimated_duration_minutes integer DEFAULT NULL::integer, p_capacity integer DEFAULT NULL::integer)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, vehicle_id uuid, license_plate character varying, default_driver_id uuid, driver_name character varying, route_name character varying, route_code character varying, direction character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, estimated_duration_minutes integer, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, capacity integer, capacity_override integer, organization_id uuid, organization_name character varying, paired_route_id uuid, seats integer, assigned_count integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_company_check(p_tenant_id, p_company_id, p_vehicle_id, p_default_driver_id);

    -- A zone PostgreSQL does not know would make every planned_start of the route a runtime error.
    IF p_timezone IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_timezone_names tz WHERE tz.name = p_timezone
    ) THEN
        RAISE EXCEPTION 'route.timezone' USING ERRCODE = 'P0001';
    END IF;

    IF NULLIF(TRIM(p_route_code), '') IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = p_company_id AND r.route_code = TRIM(p_route_code)
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.routes AS r (
        company_id, vehicle_id, default_driver_id, route_name, route_code, direction, timezone,
        origin_address, origin_lat, origin_lng,
        destination_address, destination_lat, destination_lng,
        estimated_duration_minutes, is_active, capacity
    )
    VALUES (
        p_company_id,
        p_vehicle_id,
        p_default_driver_id,
        TRIM(p_route_name),
        NULLIF(TRIM(p_route_code), ''),
        COALESCE(p_direction, 'outbound'),
        COALESCE(p_timezone, 'UTC'),
        TRIM(p_origin_address),
        p_origin_lat,
        p_origin_lng,
        TRIM(p_destination_address),
        p_destination_lat,
        p_destination_lng,
        p_estimated_duration_minutes,
        true,
        NULLIF(p_capacity, 0)
    )
    RETURNING r.id INTO v_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, v_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route(uuid,uuid,character varying,character varying,character varying,character varying,uuid,uuid,character varying,character varying,numeric,numeric,numeric,numeric,integer,integer) IS 'Creates a route for one of the tenant''s carriers and returns it as sp_get_route does; raises company.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002)';
