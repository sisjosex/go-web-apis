CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(p_company_id uuid, p_vehicle_type character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        v.plate_number,
        v.vehicle_type,
        v.brand,
        v.model,
        v.year,
        v.capacity,
        v.gps_device_id,
        v.status,
        v.created_at,
        v.updated_at
    FROM tracking.vehicles v
    WHERE
        (p_company_id IS NULL OR v.company_id = p_company_id)
        AND (p_vehicle_type IS NULL OR v.vehicle_type = p_vehicle_type)
        AND (p_status IS NULL OR v.status = p_status)
    ORDER BY v.created_at DESC;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(p_tenant_id uuid, p_company_id uuid DEFAULT NULL::uuid, p_vehicle_type character varying DEFAULT NULL::character varying, p_status character varying DEFAULT NULL::character varying, p_search character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, company_id uuid, company_name character varying, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, service_blocked boolean, created_at timestamp without time zone, updated_at timestamp without time zone, default_driver_id uuid, driver_name character varying, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(tc.name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        tracking.fn_service_blocked(tc.tenant_id, 'vehicle', v.id),
        v.created_at,
        v.updated_at,
        v.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.vehicles v
    -- Not a LEFT JOIN: company_id is NOT NULL and this join is what scopes the rows to the
    -- tenant, so an outer join would only widen the result to other tenants' vehicles.
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    LEFT JOIN tracking.drivers d ON d.id = v.default_driver_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR v.company_id = p_company_id)
      AND (p_vehicle_type IS NULL OR p_vehicle_type = '' OR v.vehicle_type = p_vehicle_type)
      AND (p_status IS NULL OR p_status = '' OR v.status = p_status)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR v.plate_number ILIKE '%' || p_search || '%'
      )
    ORDER BY v.plate_number ASC, v.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_vehicles(uuid,uuid,character varying,character varying,character varying,integer,integer) IS 'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status and a plate search; every row carries its company name, its usual driver (TRACK-034 D1), whether an expired blocking document stands against it (TRACK-016 D2) and the total_count the filters match';
