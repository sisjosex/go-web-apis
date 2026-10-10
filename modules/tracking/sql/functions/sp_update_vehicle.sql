CREATE OR REPLACE FUNCTION tracking.sp_update_vehicle(p_vehicle_id uuid, p_plate_number character varying, p_vehicle_type character varying, p_brand character varying, p_model character varying, p_year integer, p_capacity integer, p_gps_device_id character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate vehicle type if provided
    IF p_vehicle_type IS NOT NULL AND p_vehicle_type NOT IN ('bus', 'van', 'car') THEN
        RAISE EXCEPTION 'vehicle.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles tv WHERE tv.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check plate uniqueness if updated
    IF p_plate_number IS NOT NULL THEN
        IF EXISTS (
            SELECT 1 FROM tracking.vehicles tv
            WHERE tv.id != p_vehicle_id 
              AND tv.company_id = (SELECT tv2.company_id FROM tracking.vehicles tv2 WHERE tv2.id = p_vehicle_id)
              AND tv.plate_number = p_plate_number
        ) THEN
            RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    UPDATE tracking.vehicles
    SET
        plate_number = COALESCE(p_plate_number, tracking.vehicles.plate_number),
        vehicle_type = COALESCE(p_vehicle_type, tracking.vehicles.vehicle_type),
        brand = COALESCE(p_brand, tracking.vehicles.brand),
        model = COALESCE(p_model, tracking.vehicles.model),
        year = COALESCE(p_year, tracking.vehicles.year),
        capacity = COALESCE(p_capacity, tracking.vehicles.capacity),
        gps_device_id = COALESCE(p_gps_device_id, tracking.vehicles.gps_device_id),
        status = COALESCE(p_status, tracking.vehicles.status),
        updated_at = CURRENT_TIMESTAMP
    WHERE tracking.vehicles.id = p_vehicle_id
    RETURNING
        tracking.vehicles.id,
        tracking.vehicles.company_id,
        tracking.vehicles.plate_number,
        tracking.vehicles.vehicle_type,
        tracking.vehicles.brand,
        tracking.vehicles.model,
        tracking.vehicles.year,
        tracking.vehicles.capacity,
        tracking.vehicles.gps_device_id,
        tracking.vehicles.status,
        tracking.vehicles.created_at,
        tracking.vehicles.updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_update_vehicle(p_tenant_id uuid, p_vehicle_id uuid, p_plate_number character varying, p_vehicle_type character varying, p_brand character varying, p_model character varying, p_year integer, p_capacity integer, p_gps_device_id character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.vehicles v SET
        plate_number = COALESCE(NULLIF(TRIM(p_plate_number), ''), v.plate_number),
        vehicle_type = COALESCE(NULLIF(TRIM(p_vehicle_type), ''), v.vehicle_type),
        brand = NULLIF(TRIM(p_brand), ''),
        model = NULLIF(TRIM(p_model), ''),
        year = COALESCE(p_year, v.year),
        capacity = COALESCE(p_capacity, v.capacity),
        gps_device_id = NULLIF(TRIM(p_gps_device_id), ''),
        status = COALESCE(NULLIF(TRIM(p_status), ''), v.status),
        updated_at = CURRENT_TIMESTAMP
    WHERE v.id = p_vehicle_id
    RETURNING
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
        v.updated_at;
END;
$function$;
