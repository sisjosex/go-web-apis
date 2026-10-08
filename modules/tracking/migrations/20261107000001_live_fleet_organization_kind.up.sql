-- TRACK-051: the web fleet draws each vehicle as the phone does — a school bus for a school's route,
-- a bus otherwise — so each row carries its route client's kind, read through route → organization as
-- sp_trip_live does. One primary-key join in the same statement: no extra round-trip.
DROP FUNCTION tracking.sp_live_fleet(UUID, UUID);

CREATE FUNCTION tracking.sp_live_fleet(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL
)
RETURNS TABLE(
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    route_id UUID,
    route_name VARCHAR,
    lat FLOAT8,
    lng FLOAT8,
    speed REAL,
    heading REAL,
    recorded_at TIMESTAMPTZ,
    organization_kind VARCHAR
) AS $$
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        t.id,
        t.route_id,
        CAST(r.route_name AS VARCHAR),
        ST_Y(lp.location::geometry),
        ST_X(lp.location::geometry),
        lp.speed,
        lp.heading,
        lp.recorded_at,
        CAST(o.kind AS VARCHAR)
    FROM tracking.vehicle_last_position lp
    INNER JOIN tracking.vehicles v ON v.id = lp.vehicle_id
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    LEFT JOIN LATERAL (
        SELECT tr.id, tr.route_id
        FROM tracking.trips tr
        WHERE tr.vehicle_id = v.id
          AND tr.status = 'in_progress'
          AND tr.tenant_id = p_tenant_id
        ORDER BY tr.started_at DESC
        LIMIT 1
    ) t ON true
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE p_organization_id IS NULL
       OR EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.riders rd ON rd.id = k.subject_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.status <> 'cancelled'
              AND rd.organization_id = p_organization_id
        )
    ORDER BY v.plate_number;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_live_fleet(UUID, UUID) IS
'Every reporting vehicle of the tenant with its last position, trip in progress, route (id and name) and the route client''s kind; narrowed to the trips carrying riders of p_organization_id when set (TRACK-025, TRACK-028, TRACK-051)';
