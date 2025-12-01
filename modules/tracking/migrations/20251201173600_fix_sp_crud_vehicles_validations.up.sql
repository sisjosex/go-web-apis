-- Migration: fix_sp_crud_vehicles_validations
-- Module: tracking
-- Created: 2025-12-01 17:36:00
-- Description: Add validations to vehicle creation and update

DROP FUNCTION IF EXISTS tracking.sp_create_vehicle(UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, INTEGER, INTEGER, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_update_vehicle(UUID, VARCHAR, VARCHAR, VARCHAR, INTEGER, INTEGER, VARCHAR, BOOLEAN);

-- CREATE VEHICLE
CREATE FUNCTION tracking.sp_create_vehicle(
    p_company_id UUID,
    p_plate_number VARCHAR(50),
    p_vehicle_type VARCHAR(50),
    p_brand VARCHAR(100) DEFAULT NULL,
    p_model VARCHAR(100) DEFAULT NULL,
    p_year INTEGER DEFAULT NULL,
    p_capacity INTEGER DEFAULT NULL,
    p_vin VARCHAR(100) DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR(50),
    vehicle_type VARCHAR(50),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INTEGER,
    capacity INTEGER,
    vin VARCHAR(100),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    -- Validate vehicle type
    IF p_vehicle_type NOT IN ('bus', 'van', 'car') THEN
        RAISE EXCEPTION 'vehicle.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    -- Validate company exists
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies WHERE id = p_company_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate plate_number is unique within company
    IF EXISTS (
        SELECT 1 FROM tracking.vehicles 
        WHERE company_id = p_company_id AND plate_number = p_plate_number
    ) THEN
        RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.vehicles (
        company_id, plate_number, vehicle_type, brand, model, year, capacity, vin
    )
    VALUES (
        p_company_id, p_plate_number, p_vehicle_type, p_brand, p_model, p_year, p_capacity, p_vin
    )
    RETURNING
        vehicles.id, vehicles.company_id, vehicles.plate_number, vehicles.vehicle_type,
        vehicles.brand, vehicles.model, vehicles.year, vehicles.capacity, vehicles.vin,
        vehicles.is_active, vehicles.created_at, vehicles.updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE VEHICLE
CREATE FUNCTION tracking.sp_update_vehicle(
    p_vehicle_id UUID,
    p_plate_number VARCHAR(50) DEFAULT NULL,
    p_brand VARCHAR(100) DEFAULT NULL,
    p_model VARCHAR(100) DEFAULT NULL,
    p_year INTEGER DEFAULT NULL,
    p_capacity INTEGER DEFAULT NULL,
    p_vin VARCHAR(100) DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR(50),
    vehicle_type VARCHAR(50),
    brand VARCHAR(100),
    model VARCHAR(100),
    year INTEGER,
    capacity INTEGER,
    vin VARCHAR(100),
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_vehicle tracking.vehicles%ROWTYPE;
    v_company_id UUID;
BEGIN
    -- Get vehicle and company_id
    SELECT * INTO v_vehicle FROM tracking.vehicles WHERE id = p_vehicle_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_company_id := v_vehicle.company_id;

    -- Validate unique plate_number within company (if plate is being updated)
    IF p_plate_number IS NOT NULL AND p_plate_number != v_vehicle.plate_number THEN
        IF EXISTS (
            SELECT 1 FROM tracking.vehicles 
            WHERE company_id = v_company_id 
              AND plate_number = p_plate_number 
              AND id != p_vehicle_id
        ) THEN
            RAISE EXCEPTION 'vehicle.plate-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN QUERY
    UPDATE tracking.vehicles
    SET
        plate_number = COALESCE(p_plate_number, vehicles.plate_number),
        brand = COALESCE(p_brand, vehicles.brand),
        model = COALESCE(p_model, vehicles.model),
        year = COALESCE(p_year, vehicles.year),
        capacity = COALESCE(p_capacity, vehicles.capacity),
        vin = COALESCE(p_vin, vehicles.vin),
        is_active = COALESCE(p_is_active, vehicles.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE vehicles.id = p_vehicle_id
    RETURNING
        vehicles.id, vehicles.company_id, vehicles.plate_number, vehicles.vehicle_type,
        vehicles.brand, vehicles.model, vehicles.year, vehicles.capacity, vehicles.vin,
        vehicles.is_active, vehicles.created_at, vehicles.updated_at;
END;
$$ LANGUAGE plpgsql;
