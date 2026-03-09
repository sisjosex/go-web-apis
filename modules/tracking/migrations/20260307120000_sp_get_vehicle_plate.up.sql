-- Migration: sp_get_vehicle_plate
-- Module: tracking
-- Description: Stored procedure to get vehicle plate number by ID

CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_plate(
    p_vehicle_id UUID
)
RETURNS TABLE(
    plate_number VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    -- Validate vehicle exists and return plate number
    RETURN QUERY
    SELECT CAST(v.plate_number AS VARCHAR)
    FROM tracking.vehicles v
    WHERE v.id = p_vehicle_id;

    -- If no results, raise exception
    IF NOT FOUND THEN
        RAISE EXCEPTION 'tracking.vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;
