-- Reverses TRACK-021 step 1: sp_list_trips without delay_seconds, as TRACK-020 left it.

DROP FUNCTION tracking.sp_list_trips(UUID, DATE, UUID, VARCHAR, UUID, INT, INT);

-- p_date NULL is each route's own today. Every route's today lies within a day of the server's, so
-- the BETWEEN keeps the (tenant_id, service_date, status) index in play before the per-zone test.
CREATE FUNCTION tracking.sp_list_trips(
    p_tenant_id UUID,
    p_date DATE DEFAULT NULL,
    p_route_id UUID DEFAULT NULL,
    p_status VARCHAR DEFAULT NULL,
    p_organization_id UUID DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    direction VARCHAR,
    route_schedule_id UUID,
    service_date DATE,
    timezone VARCHAR,
    planned_start TIMESTAMPTZ,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_id UUID,
    driver_name VARCHAR,
    status VARCHAR,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    is_overridden BOOLEAN,
    stops_count INT,
    tasks_total INT,
    tasks_done INT,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT t.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.fn_trip_rows(p_tenant_id, NULL) t
    WHERE (p_date IS NOT NULL AND t.service_date = p_date
           OR p_date IS NULL
              AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
              AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE))
      AND (p_route_id IS NULL OR t.route_id = p_route_id)
      AND (p_status IS NULL OR t.status = p_status)
      -- An organization's trips are the ones carrying one of its riders.
      AND (p_organization_id IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.trip_stops ts
          INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
          INNER JOIN tracking.riders rd ON rd.id = k.subject_id
          WHERE ts.trip_id = t.id
            AND k.subject_type = 'passenger'
            AND rd.organization_id = p_organization_id
      ))
    ORDER BY t.planned_start ASC, t.route_name ASC, t.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_trips(UUID, DATE, UUID, VARCHAR, UUID, INT, INT) IS
'One page of a day''s trips by planned start — p_date, or each route''s own today when NULL — narrowed by route, status and an organization whose riders it carries; every row carries total_count (TRACK-020)';
