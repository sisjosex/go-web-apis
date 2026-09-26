-- TRACK-020 step 1: the three trip reads — the day's list, one trip with its stops and tasks, and a
-- trip's live status.
--
-- fn_trip_rows is the one row shape the list, the detail and every trip write answer, names and
-- progress counts included, so a client never joins routes, vehicles and drivers itself. It is SQL,
-- so the list's filters are planned into it and each row's counts are one pass over that trip's
-- tasks through the UNIQUE (trip_stop_id, ...) index.
--
-- The detail nests stops → tasks as JSONB: one round-trip for the whole timeline. The status reads
-- the same counts as sp_get_route_realtime_status, over one trip instead of the route's current one.
--
-- Every caller is denied to the organization level at the route (D3), so no SP here takes a scope.

-- ===========================================================================
-- Shape
-- ===========================================================================

-- p_ids NULL is every trip of the tenant. tasks_total leaves out cancelled tasks; tasks_done counts
-- what is no longer pending — done or no_show — so done / total is the trip's progress.
CREATE FUNCTION tracking.fn_trip_rows(
    p_tenant_id UUID,
    p_ids UUID[]
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
    updated_at TIMESTAMPTZ
) AS $$
    SELECT
        t.id,
        t.route_id,
        CAST(r.route_name AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        t.route_schedule_id,
        t.service_date,
        CAST(t.timezone AS VARCHAR),
        t.planned_start,
        t.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        t.driver_id,
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(t.status AS VARCHAR),
        t.started_at,
        t.ended_at,
        t.is_overridden,
        CAST(pr.stops AS INT),
        CAST(pr.total AS INT),
        CAST(pr.done AS INT),
        t.created_at,
        t.updated_at
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = t.driver_id
    CROSS JOIN LATERAL (
        SELECT
            COUNT(DISTINCT ts.id) AS stops,
            COUNT(k.id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(k.id) FILTER (WHERE k.status IN ('done', 'no_show')) AS done
        FROM tracking.trip_stops ts
        LEFT JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
    ) pr
    WHERE t.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR t.id = ANY(p_ids));
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_rows(UUID, UUID[]) IS
'The one row shape of a trip — route, plate and driver names, stops_count, tasks_total (not cancelled) and tasks_done (done or no_show) — for p_ids, or every trip of the tenant when NULL (TRACK-020)';

-- A trip's stops in order, each with its tasks and the rider's name: the detail's timeline.
CREATE FUNCTION tracking.fn_trip_stops_json(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS JSONB AS $$
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'id',            ts.id,
        'sequence',      ts.sequence,
        'stop_place_id', ts.stop_place_id,
        'stop_name',     sp.name,
        'planned_at',    ts.planned_at,
        'arrived_at',    ts.arrived_at,
        'departed_at',   ts.departed_at,
        'status',        ts.status,
        'tasks',         tk.tasks
    ) ORDER BY ts.sequence), '[]'::jsonb)
    FROM tracking.trip_stops ts
    INNER JOIN tracking.trips t ON t.id = ts.trip_id AND t.tenant_id = p_tenant_id
    INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
    CROSS JOIN LATERAL (
        SELECT COALESCE(jsonb_agg(jsonb_build_object(
            'id',           k.id,
            'kind',         k.kind,
            'subject_type', k.subject_type,
            'subject_id',   k.subject_id,
            'rider_name',   rd.first_name || ' ' || rd.last_name,
            'status',       k.status,
            'done_at',      k.done_at
        ) ORDER BY k.kind DESC, rd.last_name, rd.first_name, k.id), '[]'::jsonb) AS tasks
        FROM tracking.trip_stop_tasks k
        LEFT JOIN tracking.riders rd ON k.subject_type = 'passenger' AND rd.id = k.subject_id
        WHERE k.trip_stop_id = ts.id
    ) tk
    WHERE ts.trip_id = p_trip_id;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_trip_stops_json(UUID, UUID) IS
'A trip''s stops by sequence, each with its tasks (pickups first) and the passenger''s name, as the JSONB array the trip detail answers (TRACK-020)';

-- ===========================================================================
-- List
-- ===========================================================================

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

-- ===========================================================================
-- Detail
-- ===========================================================================

CREATE FUNCTION tracking.sp_get_trip(
    p_tenant_id UUID,
    p_trip_id UUID
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
    stops JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT t.*, tracking.fn_trip_stops_json(p_tenant_id, t.id)
    FROM tracking.fn_trip_rows(p_tenant_id, ARRAY[p_trip_id]) t;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_trip(UUID, UUID) IS
'One trip in the list''s row shape plus its stops and their tasks as JSONB, in one round-trip; raises trip.not-found (TRACK-020)';

-- ===========================================================================
-- Status
-- ===========================================================================

-- The counts are sp_get_route_realtime_status's, over this trip. The next stop is the first one still
-- pending while the trip is planned or in progress, and the delay is how long it has been due (D2:
-- projected, now − its planned_at, never negative); both are NULL once the trip has ended.
CREATE FUNCTION tracking.sp_get_trip_status(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(
    trip_id UUID,
    route_id UUID,
    status VARCHAR,
    planned_start TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    next_stop JSONB,
    delay_seconds INT,
    vehicle_id UUID,
    current_latitude DECIMAL,
    current_longitude DECIMAL,
    current_speed DECIMAL,
    location_age_seconds INT,
    total_riders INT,
    boarded_count INT,
    arrived_count INT,
    no_show_count INT,
    pending_count INT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.route_id,
        CAST(t.status AS VARCHAR),
        t.planned_start,
        t.started_at,
        t.ended_at,
        CASE WHEN ns.id IS NOT NULL THEN jsonb_build_object(
            'id',         ns.id,
            'sequence',   ns.sequence,
            'stop_name',  ns.stop_name,
            'planned_at', ns.planned_at
        ) END,
        CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - ns.planned_at))) AS INT),
        t.vehicle_id,
        vl.latitude,
        vl.longitude,
        vl.speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT),
        CAST(COALESCE(tk.total, 0) AS INT),
        CAST(COALESCE(tk.boarded, 0) AS INT),
        CAST(COALESCE(tk.arrived, 0) AS INT),
        CAST(COALESCE(tk.no_show, 0) AS INT),
        CAST(COALESCE(tk.total - tk.touched, 0) AS INT)
    FROM tracking.trips t
    LEFT JOIN LATERAL (
        SELECT ts.id, ts.sequence, sp.name AS stop_name, ts.planned_at
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND ts.status = 'pending'
          AND t.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON true
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = t.vehicle_id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status <> 'cancelled') AS total,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'done') AS boarded,
            COUNT(*) FILTER (WHERE k.kind = 'dropoff' AND k.status = 'done') AS arrived,
            COUNT(*) FILTER (WHERE k.kind = 'pickup' AND k.status = 'no_show') AS no_show,
            COUNT(DISTINCT k.subject_id) FILTER (WHERE k.status IN ('done', 'no_show')) AS touched
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE ts.trip_id = t.id
    ) tk ON true
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_trip_status(UUID, UUID) IS
'A trip''s live status: rider counts as sp_get_route_realtime_status, the next pending stop, the delay since it was due (D2, projected) and the vehicle''s last position; raises trip.not-found (TRACK-020)';
