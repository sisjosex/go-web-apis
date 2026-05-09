-- Revert duplicate check: restore sp_assign_rider without duplicate check

CREATE OR REPLACE FUNCTION tracking.sp_assign_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_pickup_stop_id UUID,
    p_dropoff_stop_id UUID,
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_rider_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.rider_assignments (
        rider_id, route_id, pickup_stop_id, dropoff_stop_id, status
    )
    VALUES (
        p_rider_id,
        p_route_id,
        p_pickup_stop_id,
        p_dropoff_stop_id,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.rider_assignments.id,
        tracking.rider_assignments.rider_id,
        tracking.rider_assignments.route_id,
        tracking.rider_assignments.pickup_stop_id,
        tracking.rider_assignments.dropoff_stop_id,
        CAST(tracking.rider_assignments.status AS VARCHAR),
        tracking.rider_assignments.created_at,
        tracking.rider_assignments.updated_at;
END;
$$ LANGUAGE plpgsql;
