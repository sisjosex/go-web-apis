-- Migration: sp_crud_assignments
-- Module: tracking
-- Created: 2025-11-30 15:13:21

-- Rider Assignments CRUD
CREATE OR REPLACE FUNCTION tracking.sp_assign_rider(p_rider_id UUID, p_route_id UUID, p_pickup_stop_id UUID DEFAULT NULL, p_dropoff_stop_id UUID DEFAULT NULL)
RETURNS TABLE(id UUID, rider_id UUID, route_id UUID, pickup_stop_id UUID, dropoff_stop_id UUID, is_active BOOLEAN, assigned_at TIMESTAMP, unassigned_at TIMESTAMP, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY INSERT INTO tracking.rider_assignments (rider_id, route_id, pickup_stop_id, dropoff_stop_id)
    VALUES (p_rider_id, p_route_id, p_pickup_stop_id, p_dropoff_stop_id)
    RETURNING rider_assignments.id, rider_assignments.rider_id, rider_assignments.route_id, rider_assignments.pickup_stop_id, rider_assignments.dropoff_stop_id, rider_assignments.is_active, rider_assignments.assigned_at, rider_assignments.unassigned_at, rider_assignments.created_at, rider_assignments.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_unassign_rider(p_assignment_id UUID) RETURNS BOOLEAN AS $$
BEGIN
    UPDATE tracking.rider_assignments SET is_active = false, unassigned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = p_assignment_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'assignment_not_found'; END IF;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_rider_assignments(p_rider_id UUID DEFAULT NULL, p_route_id UUID DEFAULT NULL, p_is_active BOOLEAN DEFAULT NULL)
RETURNS TABLE(id UUID, rider_id UUID, route_id UUID, pickup_stop_id UUID, dropoff_stop_id UUID, is_active BOOLEAN, assigned_at TIMESTAMP, unassigned_at TIMESTAMP, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT a.id, a.rider_id, a.route_id, a.pickup_stop_id, a.dropoff_stop_id, a.is_active, a.assigned_at, a.unassigned_at, a.created_at, a.updated_at
    FROM tracking.rider_assignments a WHERE (p_rider_id IS NULL OR a.rider_id = p_rider_id) AND (p_route_id IS NULL OR a.route_id = p_route_id) AND (p_is_active IS NULL OR a.is_active = p_is_active) ORDER BY a.created_at DESC;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

