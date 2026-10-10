CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle(p_vehicle_id uuid)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles WHERE tracking.vehicles.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

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
    WHERE v.id = p_vehicle_id;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle(p_tenant_id uuid, p_vehicle_id uuid)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone, default_driver_id uuid, driver_name character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        v.created_at,
        v.updated_at,
        v.default_driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR)
    FROM tracking.vehicles v
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    LEFT JOIN tracking.drivers d ON d.id = v.default_driver_id
    WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_vehicle(uuid,uuid) IS 'One vehicle of the tenant with its usual driver; raises vehicle.not-found (TRACK-034 D1)';
