-- Migration: fix_sp_update_route_types
-- Module: tracking
-- Created: 2025-12-06 16:33:29

-- Fix data type mismatch in sp_update_route RETURNS TABLE
-- origin_address and destination_address are VARCHAR(500), not TEXT

DROP FUNCTION IF EXISTS tracking.sp_update_route(UUID, VARCHAR, UUID, BOOLEAN);

CREATE OR REPLACE FUNCTION tracking.sp_update_route(
    p_route_id UUID,
    p_route_name VARCHAR DEFAULT NULL,
    p_vehicle_id UUID DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    company_id UUID,
    vehicle_id UUID,
    route_name VARCHAR(255),
    route_code VARCHAR(50),
    origin_address VARCHAR(500),
    origin_lat DECIMAL(10, 8),
    origin_lng DECIMAL(11, 8),
    destination_address VARCHAR(500),
    destination_lat DECIMAL(10, 8),
    destination_lng DECIMAL(11, 8),
    schedule_type VARCHAR(50),
    scheduled_start_time TIME,
    scheduled_end_time TIME,
    estimated_duration_minutes INTEGER,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
)
LANGUAGE plpgsql
AS $$
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
$$;
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

