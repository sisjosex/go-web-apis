-- Migration: sp_vehicle_list_get_delete
-- Module: tracking
-- Created: 2025-12-06 13:00:48

-- LIST VEHICLES
CREATE OR REPLACE FUNCTION tracking.sp_list_vehicles(
    p_company_id UUID,
    p_vehicle_type VARCHAR(50),
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
$$ LANGUAGE plpgsql;

-- GET VEHICLE BY ID
CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle(
    p_vehicle_id UUID
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
$$ LANGUAGE plpgsql;

-- DELETE VEHICLE
CREATE OR REPLACE FUNCTION tracking.sp_delete_vehicle(
    p_vehicle_id UUID
)
RETURNS BOOLEAN AS $$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles WHERE tracking.vehicles.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the vehicle (CASCADE will handle related records)
    DELETE FROM tracking.vehicles WHERE tracking.vehicles.id = p_vehicle_id;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
-- Example table creation:
-- CREATE TABLE IF NOT EXISTS tracking.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION tracking.sp_operation() RETURNS TABLE(...) AS $$
-- BEGIN
--     -- Logic here
-- END;
-- $$ LANGUAGE plpgsql;

