-- Migration: fix_sp_create_route_stop_params
-- Module: tracking
-- Created: 2025-12-06 16:40:26

-- Fix sp_create_route_stop parameter order and names to match repository call
-- Repository sends: route_id, stop_name, stop_address, latitude, longitude, sequence_order, scheduled_time

DROP FUNCTION IF EXISTS tracking.sp_create_route_stop(UUID, INT, VARCHAR, DECIMAL, DECIMAL, TIME, VARCHAR);

CREATE OR REPLACE FUNCTION tracking.sp_create_route_stop(
    p_route_id UUID,
    p_stop_name VARCHAR(255),
    p_stop_address VARCHAR(500),
    p_latitude DECIMAL(10, 8),
    p_longitude DECIMAL(11, 8),
    p_sequence_order INT,
    p_scheduled_time VARCHAR DEFAULT NULL  -- HH:MM:SS format
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    stop_name VARCHAR(255),
    address VARCHAR(255),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    stop_order INT,
    scheduled_arrival_offset_minutes INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_offset_minutes INT := NULL;
    v_scheduled_time TIME := NULL;
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes WHERE tracking.routes.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Convert scheduled_time string to TIME and calculate offset
    IF p_scheduled_time IS NOT NULL AND p_scheduled_time != '' THEN
        BEGIN
            v_scheduled_time := p_scheduled_time::TIME;
            -- Calculate offset in minutes from midnight
            v_offset_minutes := EXTRACT(HOUR FROM v_scheduled_time) * 60 + EXTRACT(MINUTE FROM v_scheduled_time);
        EXCEPTION WHEN OTHERS THEN
            RAISE EXCEPTION 'route-stop.invalid-time-format' USING ERRCODE = 'P0001';
        END;
    END IF;

    RETURN QUERY
    INSERT INTO tracking.route_stops (
        route_id, 
        location_name,
        latitude, 
        longitude, 
        stop_order,
        estimated_arrival,
        status
    )
    VALUES (
        p_route_id, 
        p_stop_name,
        p_latitude, 
        p_longitude, 
        p_sequence_order,
        v_scheduled_time,
        'active'
    )
    RETURNING
        tracking.route_stops.id,
        tracking.route_stops.route_id,
        tracking.route_stops.location_name,
        tracking.route_stops.location_name,  -- Use location_name for address field too
        tracking.route_stops.latitude,
        tracking.route_stops.longitude,
        tracking.route_stops.stop_order,
        v_offset_minutes,
        tracking.route_stops.created_at,
        tracking.route_stops.updated_at;
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

