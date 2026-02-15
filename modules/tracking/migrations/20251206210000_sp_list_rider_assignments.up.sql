-- List Rider Assignments
CREATE OR REPLACE FUNCTION tracking.sp_list_rider_assignments(
    p_rider_id UUID,
    p_route_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ra.id,
        ra.rider_id,
        ra.route_id,
        ra.pickup_stop_id,
        ra.dropoff_stop_id,
        ra.status,
        ra.created_at,
        ra.updated_at
    FROM tracking.rider_assignments ra
    WHERE
        (p_rider_id IS NULL OR ra.rider_id = p_rider_id)
        AND (p_route_id IS NULL OR ra.route_id = p_route_id)
        AND (p_is_active IS NULL OR (p_is_active = TRUE AND ra.status = 'active') OR (p_is_active = FALSE AND ra.status != 'active'))
    ORDER BY ra.created_at DESC;
END;
$$ LANGUAGE plpgsql;
