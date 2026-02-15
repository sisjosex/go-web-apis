-- CREATE VEHICLE
CREATE OR REPLACE FUNCTION tracking.sp_create_vehicle(
    p_company_id UUID,
    p_plate_number VARCHAR(50),
    p_vehicle_type VARCHAR(50),
    p_brand VARCHAR(100),
    p_model VARCHAR(100),
    p_year INT,
    p_capacity INT,
    p_gps_device_id VARCHAR(100),
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR(50),
    vehicle_type VARCHAR(50),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INT,
    capacity INT,
    gps_device_id VARCHAR(100),
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
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
$$ LANGUAGE plpgsql;

-- UPDATE VEHICLE
CREATE OR REPLACE FUNCTION tracking.sp_update_vehicle(
    p_vehicle_id UUID,
    p_plate_number VARCHAR(50),
    p_vehicle_type VARCHAR(50),
    p_brand VARCHAR(100),
    p_model VARCHAR(100),
    p_year INT,
    p_capacity INT,
    p_gps_device_id VARCHAR(100),
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR(50),
    vehicle_type VARCHAR(50),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INT,
    capacity INT,
    gps_device_id VARCHAR(100),
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
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
$$ LANGUAGE plpgsql;
