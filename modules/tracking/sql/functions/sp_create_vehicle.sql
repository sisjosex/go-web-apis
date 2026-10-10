CREATE OR REPLACE FUNCTION tracking.sp_create_vehicle(p_company_id uuid, p_plate_number character varying, p_vehicle_type character varying, p_brand character varying, p_model character varying, p_year integer, p_capacity integer, p_gps_device_id character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate vehicle type
    IF p_vehicle_type NOT IN ('bus', 'van', 'car') THEN
        RAISE EXCEPTION 'vehicle.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Check if company exists
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc 
        WHERE tc.id = p_company_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if plate already exists in this company
    IF EXISTS (
        SELECT 1 FROM tracking.vehicles tv
        WHERE tv.company_id = p_company_id 
          AND tv.plate_number = p_plate_number
    ) THEN
        RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Insert vehicle
    RETURN QUERY
    INSERT INTO tracking.vehicles (
        company_id, plate_number, vehicle_type, brand, model, 
        year, capacity, gps_device_id, status, created_at, updated_at
    )
    VALUES (
        p_company_id, p_plate_number, p_vehicle_type, p_brand, p_model,
        p_year, p_capacity, p_gps_device_id, p_status, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
    )
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

CREATE OR REPLACE FUNCTION tracking.sp_create_vehicle(p_tenant_id uuid, p_company_id uuid, p_plate_number character varying, p_vehicle_type character varying, p_brand character varying, p_model character varying, p_year integer, p_capacity integer, p_gps_device_id character varying, p_status character varying)
 RETURNS TABLE(id uuid, company_id uuid, plate_number character varying, vehicle_type character varying, brand character varying, model character varying, year integer, capacity integer, gps_device_id character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.vehicles v
        WHERE v.company_id = p_company_id
          AND v.plate_number = TRIM(p_plate_number)
    ) THEN
        RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.vehicles (
        company_id, plate_number, vehicle_type, brand, model, year, capacity, gps_device_id, status
    )
    VALUES (
        p_company_id,
        TRIM(p_plate_number),
        p_vehicle_type,
        NULLIF(TRIM(p_brand), ''),
        NULLIF(TRIM(p_model), ''),
        p_year,
        COALESCE(p_capacity, 0),
        NULLIF(TRIM(p_gps_device_id), ''),
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.vehicles.id,
        tracking.vehicles.company_id,
        CAST(tracking.vehicles.plate_number AS VARCHAR),
        CAST(tracking.vehicles.vehicle_type AS VARCHAR),
        CAST(tracking.vehicles.brand AS VARCHAR),
        CAST(tracking.vehicles.model AS VARCHAR),
        tracking.vehicles.year,
        tracking.vehicles.capacity,
        CAST(tracking.vehicles.gps_device_id AS VARCHAR),
        CAST(tracking.vehicles.status AS VARCHAR),
        tracking.vehicles.created_at,
        tracking.vehicles.updated_at;
END;
$function$;
