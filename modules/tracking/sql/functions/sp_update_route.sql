CREATE OR REPLACE FUNCTION tracking.sp_update_route(p_route_id uuid, p_route_name character varying DEFAULT NULL::character varying, p_vehicle_id uuid DEFAULT NULL::uuid, p_is_active boolean DEFAULT NULL::boolean)
 RETURNS TABLE(id uuid, company_id uuid, vehicle_id uuid, route_name character varying, route_code character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, schedule_type character varying, scheduled_start_time time without time zone, scheduled_end_time time without time zone, estimated_duration_minutes integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE tracking.routes.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Return updated route
    RETURN QUERY
    UPDATE tracking.routes
    SET
        route_name = COALESCE(p_route_name, tracking.routes.route_name),
        vehicle_id = COALESCE(p_vehicle_id, tracking.routes.vehicle_id),
        is_active = COALESCE(p_is_active, tracking.routes.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE tracking.routes.id = p_route_id
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

CREATE OR REPLACE FUNCTION tracking.sp_update_route(p_tenant_id uuid, p_route_id uuid, p_route_name character varying DEFAULT NULL::character varying, p_route_code character varying DEFAULT NULL::character varying, p_direction character varying DEFAULT NULL::character varying, p_vehicle_id uuid DEFAULT NULL::uuid, p_default_driver_id uuid DEFAULT NULL::uuid, p_timezone character varying DEFAULT NULL::character varying, p_origin_address character varying DEFAULT NULL::character varying, p_origin_lat numeric DEFAULT NULL::numeric, p_origin_lng numeric DEFAULT NULL::numeric, p_destination_address character varying DEFAULT NULL::character varying, p_destination_lat numeric DEFAULT NULL::numeric, p_destination_lng numeric DEFAULT NULL::numeric, p_estimated_duration_minutes integer DEFAULT NULL::integer, p_is_active boolean DEFAULT NULL::boolean, p_capacity integer DEFAULT NULL::integer)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, vehicle_id uuid, license_plate character varying, default_driver_id uuid, driver_name character varying, route_name character varying, route_code character varying, direction character varying, origin_address character varying, origin_lat numeric, origin_lng numeric, destination_address character varying, destination_lat numeric, destination_lng numeric, estimated_duration_minutes integer, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, capacity integer, capacity_override integer, organization_id uuid, organization_name character varying, paired_route_id uuid, seats integer, assigned_count integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_company_id UUID;
    v_vehicle_id UUID;
    v_driver_id  UUID;
BEGIN
    SELECT r.company_id, r.vehicle_id, r.default_driver_id INTO v_company_id, v_vehicle_id, v_driver_id
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;

    IF v_company_id IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_route_company_check(p_tenant_id, v_company_id, p_vehicle_id, p_default_driver_id);
    -- TRACK-064 D1: only a newly assigned one is refused, so an edit of a route whose bus expired
    -- yesterday still saves.
    PERFORM tracking.fn_service_block_check(p_tenant_id,
        CASE WHEN p_vehicle_id IS DISTINCT FROM v_vehicle_id THEN p_vehicle_id END,
        CASE WHEN p_default_driver_id IS DISTINCT FROM v_driver_id THEN p_default_driver_id END,
        'default_driver_id');

    IF p_timezone IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_timezone_names tz WHERE tz.name = p_timezone
    ) THEN
        RAISE EXCEPTION 'route.timezone' USING ERRCODE = 'P0001';
    END IF;

    IF NULLIF(TRIM(p_route_code), '') IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.routes r
        WHERE r.company_id = v_company_id
          AND r.route_code = TRIM(p_route_code)
          AND r.id <> p_route_id
    ) THEN
        RAISE EXCEPTION 'route.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.routes r SET
        route_name = COALESCE(NULLIF(TRIM(p_route_name), ''), r.route_name),
        route_code = CASE WHEN p_route_code IS NULL THEN r.route_code ELSE NULLIF(TRIM(p_route_code), '') END,
        direction = COALESCE(p_direction, r.direction),
        vehicle_id = p_vehicle_id,
        default_driver_id = p_default_driver_id,
        timezone = COALESCE(p_timezone, r.timezone),
        origin_address = COALESCE(NULLIF(TRIM(p_origin_address), ''), r.origin_address),
        origin_lat = COALESCE(p_origin_lat, r.origin_lat),
        origin_lng = COALESCE(p_origin_lng, r.origin_lng),
        destination_address = COALESCE(NULLIF(TRIM(p_destination_address), ''), r.destination_address),
        destination_lat = COALESCE(p_destination_lat, r.destination_lat),
        destination_lng = COALESCE(p_destination_lng, r.destination_lng),
        estimated_duration_minutes = COALESCE(p_estimated_duration_minutes, r.estimated_duration_minutes),
        is_active = COALESCE(p_is_active, r.is_active),
        -- NULL keeps the override, 0 clears it back to the vehicle's capacity.
        capacity = CASE WHEN p_capacity IS NULL THEN r.capacity ELSE NULLIF(p_capacity, 0) END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_route_id;

    RETURN QUERY SELECT * FROM tracking.sp_get_route(p_tenant_id, p_route_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_route(uuid,uuid,character varying,character varying,character varying,uuid,uuid,character varying,character varying,numeric,numeric,character varying,numeric,numeric,integer,boolean,integer) IS 'Edits a route and returns it as sp_get_route does; company_id is immutable, vehicle and default driver are written as sent (NULL clears), the rest keep their value when NULL. Raises route.not-found, route.company-mismatch, route.timezone or route.code-already-exists (TRACK-002), service.blocked for a newly assigned vehicle or driver (TRACK-064)';
