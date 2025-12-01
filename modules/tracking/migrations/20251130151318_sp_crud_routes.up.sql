-- Migration: sp_crud_routes
-- Module: tracking
-- Created: 2025-11-30 15:13:18

-- ============================================================================
-- ROUTES CRUD STORED PROCEDURES
-- ============================================================================

-- CREATE ROUTE
CREATE OR REPLACE FUNCTION tracking.sp_create_route(
    p_company_id UUID,
    p_route_name VARCHAR(255),
    p_origin_address TEXT,
    p_destination_address TEXT,
    p_route_code VARCHAR(50) DEFAULT NULL,
    p_vehicle_id UUID DEFAULT NULL,
    p_origin_lat DECIMAL(10,8) DEFAULT NULL,
    p_origin_lng DECIMAL(11,8) DEFAULT NULL,
    p_destination_lat DECIMAL(10,8) DEFAULT NULL,
    p_destination_lng DECIMAL(11,8) DEFAULT NULL,
    p_schedule_type VARCHAR(50) DEFAULT 'morning',
    p_scheduled_start_time TIME DEFAULT NULL,
    p_scheduled_end_time TIME DEFAULT NULL,
    p_estimated_duration_minutes INTEGER DEFAULT NULL
)
RETURNS TABLE(
    id UUID, company_id UUID, vehicle_id UUID, route_name VARCHAR(255), route_code VARCHAR(50),
    origin_address TEXT, origin_lat DECIMAL(10,8), origin_lng DECIMAL(11,8), destination_address TEXT,
    destination_lat DECIMAL(10,8), destination_lng DECIMAL(11,8), schedule_type VARCHAR(50), scheduled_start_time TIME,
    scheduled_end_time TIME, estimated_duration_minutes INTEGER, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    INSERT INTO tracking.routes (
        company_id, route_name, origin_address, destination_address, route_code, vehicle_id,
        origin_lat, origin_lng, destination_lat, destination_lng, schedule_type,
        scheduled_start_time, scheduled_end_time, estimated_duration_minutes
    ) VALUES (
        p_company_id, p_route_name, p_origin_address, p_destination_address, p_route_code, p_vehicle_id,
        p_origin_lat, p_origin_lng, p_destination_lat, p_destination_lng, p_schedule_type,
        p_scheduled_start_time, p_scheduled_end_time, p_estimated_duration_minutes
    )
    RETURNING routes.id, routes.company_id, routes.vehicle_id, routes.route_name, routes.route_code,
        routes.origin_address, routes.origin_lat, routes.origin_lng, routes.destination_address,
        routes.destination_lat, routes.destination_lng, routes.schedule_type, routes.scheduled_start_time,
        routes.scheduled_end_time, routes.estimated_duration_minutes, routes.is_active, routes.created_at, routes.updated_at;
END;
$$ LANGUAGE plpgsql;

-- UPDATE/LIST/GET/DELETE Routes (simplified for brevity)
CREATE OR REPLACE FUNCTION tracking.sp_update_route(p_route_id UUID, p_route_name VARCHAR(255) DEFAULT NULL, p_vehicle_id UUID DEFAULT NULL, p_is_active BOOLEAN DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, vehicle_id UUID, route_name VARCHAR(255), route_code VARCHAR(50), origin_address TEXT, origin_lat DECIMAL(10,8), origin_lng DECIMAL(11,8), destination_address TEXT, destination_lat DECIMAL(10,8), destination_lng DECIMAL(11,8), schedule_type VARCHAR(50), scheduled_start_time TIME, scheduled_end_time TIME, estimated_duration_minutes INTEGER, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY UPDATE tracking.routes SET route_name = COALESCE(p_route_name, routes.route_name), vehicle_id = COALESCE(p_vehicle_id, routes.vehicle_id), is_active = COALESCE(p_is_active, routes.is_active), updated_at = CURRENT_TIMESTAMP WHERE routes.id = p_route_id
    RETURNING routes.id, routes.company_id, routes.vehicle_id, routes.route_name, routes.route_code, routes.origin_address, routes.origin_lat, routes.origin_lng, routes.destination_address, routes.destination_lat, routes.destination_lng, routes.schedule_type, routes.scheduled_start_time, routes.scheduled_end_time, routes.estimated_duration_minutes, routes.is_active, routes.created_at, routes.updated_at;
    IF NOT FOUND THEN RAISE EXCEPTION 'route_not_found'; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_routes(p_company_id UUID DEFAULT NULL, p_is_active BOOLEAN DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, vehicle_id UUID, route_name VARCHAR(255), route_code VARCHAR(50), origin_address TEXT, origin_lat DECIMAL(10,8), origin_lng DECIMAL(11,8), destination_address TEXT, destination_lat DECIMAL(10,8), destination_lng DECIMAL(11,8), schedule_type VARCHAR(50), scheduled_start_time TIME, scheduled_end_time TIME, estimated_duration_minutes INTEGER, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT r.id, r.company_id, r.vehicle_id, r.route_name, r.route_code, r.origin_address, r.origin_lat, r.origin_lng, r.destination_address, r.destination_lat, r.destination_lng, r.schedule_type, r.scheduled_start_time, r.scheduled_end_time, r.estimated_duration_minutes, r.is_active, r.created_at, r.updated_at FROM tracking.routes r WHERE (p_company_id IS NULL OR r.company_id = p_company_id) AND (p_is_active IS NULL OR r.is_active = p_is_active) ORDER BY r.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_get_route(p_route_id UUID)
RETURNS TABLE(id UUID, company_id UUID, vehicle_id UUID, route_name VARCHAR(255), route_code VARCHAR(50), origin_address TEXT, origin_lat DECIMAL(10,8), origin_lng DECIMAL(11,8), destination_address TEXT, destination_lat DECIMAL(10,8), destination_lng DECIMAL(11,8), schedule_type VARCHAR(50), scheduled_start_time TIME, scheduled_end_time TIME, estimated_duration_minutes INTEGER, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT r.id, r.company_id, r.vehicle_id, r.route_name, r.route_code, r.origin_address, r.origin_lat, r.origin_lng, r.destination_address, r.destination_lat, r.destination_lng, r.schedule_type, r.scheduled_start_time, r.scheduled_end_time, r.estimated_duration_minutes, r.is_active, r.created_at, r.updated_at FROM tracking.routes r WHERE r.id = p_route_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'route_not_found'; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_route(p_route_id UUID) RETURNS BOOLEAN AS $$
BEGIN
    UPDATE tracking.routes SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = p_route_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'route_not_found'; END IF;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

