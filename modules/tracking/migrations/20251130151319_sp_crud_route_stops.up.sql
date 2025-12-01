-- Migration: sp_crud_route_stops
-- Module: tracking
-- Created: 2025-11-30 15:13:19

-- Route Stops CRUD
CREATE OR REPLACE FUNCTION tracking.sp_create_route_stop(p_route_id UUID, p_stop_name VARCHAR(255), p_stop_address TEXT, p_latitude DECIMAL(10,8), p_longitude DECIMAL(11,8), p_sequence_order INTEGER, p_scheduled_time TIME DEFAULT NULL)
RETURNS TABLE(id UUID, route_id UUID, stop_name VARCHAR(255), stop_address TEXT, latitude DECIMAL(10,8), longitude DECIMAL(11,8), sequence_order INTEGER, scheduled_time TIME, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY INSERT INTO tracking.route_stops (route_id, stop_name, stop_address, latitude, longitude, sequence_order, scheduled_time)
    VALUES (p_route_id, p_stop_name, p_stop_address, p_latitude, p_longitude, p_sequence_order, p_scheduled_time)
    RETURNING route_stops.id, route_stops.route_id, route_stops.stop_name, route_stops.stop_address, route_stops.latitude, route_stops.longitude, route_stops.sequence_order, route_stops.scheduled_time, route_stops.created_at, route_stops.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_route_stops(p_route_id UUID)
RETURNS TABLE(id UUID, route_id UUID, stop_name VARCHAR(255), stop_address TEXT, latitude DECIMAL(10,8), longitude DECIMAL(11,8), sequence_order INTEGER, scheduled_time TIME, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT s.id, s.route_id, s.stop_name, s.stop_address, s.latitude, s.longitude, s.sequence_order, s.scheduled_time, s.created_at, s.updated_at
    FROM tracking.route_stops s WHERE s.route_id = p_route_id ORDER BY s.sequence_order ASC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_route_stop(p_stop_id UUID) RETURNS BOOLEAN AS $$
BEGIN
    DELETE FROM tracking.route_stops WHERE id = p_stop_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'stop_not_found'; END IF;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

