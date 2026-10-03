-- TRACK-034 D1 rollback: routes get their driver back from the vehicle, then the vehicle column goes.

UPDATE tracking.routes r SET default_driver_id = v.default_driver_id
FROM tracking.vehicles v
WHERE v.id = r.vehicle_id AND r.default_driver_id IS NULL AND v.default_driver_id IS NOT NULL;

DROP FUNCTION tracking.sp_set_vehicle_driver(UUID, UUID, UUID);
DROP FUNCTION tracking.sp_get_vehicle(UUID, UUID);
DROP FUNCTION tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT);

CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle(
    p_tenant_id UUID,
    p_vehicle_id UUID
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    plate_number VARCHAR,
    vehicle_type VARCHAR,
    brand VARCHAR,
    model VARCHAR,
    year INT,
    capacity INT,
    gps_device_id VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        v.created_at,
        v.updated_at
    FROM tracking.vehicles v
    WHERE v.id = p_vehicle_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_vehicles(
    p_tenant_id    UUID,
    p_company_id   UUID    DEFAULT NULL,
    p_vehicle_type VARCHAR DEFAULT NULL,
    p_status       VARCHAR DEFAULT NULL,
    p_search       VARCHAR DEFAULT NULL,
    p_page         INT     DEFAULT 1,
    p_page_size    INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    plate_number VARCHAR,
    vehicle_type VARCHAR,
    brand VARCHAR,
    model VARCHAR,
    year INT,
    capacity INT,
    gps_device_id VARCHAR,
    status VARCHAR,
    service_blocked BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(tc.name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        tracking.fn_service_blocked(tc.tenant_id, 'vehicle', v.id),
        v.created_at,
        v.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.vehicles v
    -- Not a LEFT JOIN: company_id is NOT NULL and this join is what scopes the rows to the
    -- tenant, so an outer join would only widen the result to other tenants' vehicles.
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR v.company_id = p_company_id)
      AND (p_vehicle_type IS NULL OR p_vehicle_type = '' OR v.vehicle_type = p_vehicle_type)
      AND (p_status IS NULL OR p_status = '' OR v.status = p_status)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR v.plate_number ILIKE '%' || p_search || '%'
      )
    -- Alphabetical by plate, for the same reason as the company list above.
    ORDER BY v.plate_number ASC, v.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT) IS
'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status and a plate search; every row carries its company name, whether an expired blocking document stands against it (TRACK-016 D2) and the total_count the filters match. p_company_id NULL lists the whole fleet (TRACK-001 D2)';

CREATE OR REPLACE FUNCTION tracking.sp_preview_route(
    p_tenant_id UUID,
    p_route_id UUID,
    p_from DATE,
    p_to DATE
)
RETURNS TABLE(
    service_date DATE,
    schedule_id UUID,
    start_time VARCHAR,
    version_id UUID,
    vehicle_id UUID,
    driver_id UUID,
    status VARCHAR,
    exceptions JSONB
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_to < p_from OR (p_to - p_from) > 91 THEN
        RAISE EXCEPTION 'route.preview-range' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    WITH days AS (
        SELECT CAST(d AS DATE) AS service_date
        FROM generate_series(p_from, p_to, INTERVAL '1 day') d
    ),
    -- One row per departure the recurrence produces. The weekday test is the bitmask of D1 —
    -- ISODOW is 1 for Monday, so bit (ISODOW - 1) — and a special_service date adds a departure the
    -- mask would not have produced, while a no_service date removes every one of them.
    runs AS (
        SELECT dy.service_date, s.id AS schedule_id, s.start_time
        FROM days dy
        INNER JOIN tracking.route_schedules s
                ON s.route_id = p_route_id
               AND dy.service_date >= s.valid_from
               AND (s.valid_until IS NULL OR dy.service_date <= s.valid_until)
        LEFT JOIN tracking.calendar_dates cd
               ON cd.calendar_id = s.calendar_id AND cd.date = dy.service_date
        WHERE (
                ((s.days_of_week >> (CAST(EXTRACT(ISODOW FROM dy.service_date) AS INT) - 1)) & 1) = 1
                OR cd.kind = 'special_service'
              )
          AND cd.kind IS DISTINCT FROM 'no_service'
    ),
    -- Exceptions belong to the route-day, not to one departure: a day off is a day off whichever
    -- schedule produced the run. One pass over the route's exceptions, not one per day.
    applied AS (
        SELECT
            r.service_date,
            jsonb_agg(jsonb_build_object(
                'id', e.id, 'kind', e.kind, 'payload', e.payload, 'reason', e.reason
            ) ORDER BY e.created_at) AS exceptions,
            bool_or(e.kind = 'cancel') AS cancelled,
            MAX(e.payload->>'start_time') FILTER (WHERE e.kind = 'change_time')  AS start_time_override,
            MAX(e.payload->>'vehicle_id') FILTER (WHERE e.kind = 'change_vehicle') AS vehicle_override,
            MAX(e.payload->>'driver_id')  FILTER (WHERE e.kind = 'change_driver')  AS driver_override
        FROM (SELECT DISTINCT r2.service_date FROM runs r2) r
        INNER JOIN tracking.route_exceptions e
                ON e.route_id = p_route_id
               AND r.service_date BETWEEN e.date_from AND e.date_to
        GROUP BY r.service_date
    )
    SELECT
        rn.service_date,
        rn.schedule_id,
        CAST(COALESCE(ap.start_time_override, TO_CHAR(rn.start_time, 'HH24:MI')) AS VARCHAR),
        rv.id,
        COALESCE(CAST(ap.vehicle_override AS UUID), rt.vehicle_id),
        COALESCE(CAST(ap.driver_override AS UUID), rt.default_driver_id),
        CAST(CASE WHEN COALESCE(ap.cancelled, FALSE) THEN 'cancelled' ELSE 'planned' END AS VARCHAR),
        COALESCE(ap.exceptions, '[]'::jsonb)
    FROM runs rn
    INNER JOIN tracking.routes rt ON rt.id = p_route_id
    LEFT JOIN applied ap ON ap.service_date = rn.service_date
    -- The stop list in force that day, so the plan names the version a trip would run (D3).
    LEFT JOIN tracking.route_versions rv
           ON rv.route_id = p_route_id
          AND rn.service_date >= rv.effective_from
          AND (rv.effective_to IS NULL OR rn.service_date <= rv.effective_to)
    ORDER BY rn.service_date ASC, rn.start_time ASC;
END;
$$ LANGUAGE plpgsql STABLE;


ALTER TABLE tracking.vehicles DROP COLUMN default_driver_id;
