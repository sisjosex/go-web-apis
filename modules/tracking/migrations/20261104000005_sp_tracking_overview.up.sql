-- TRACK-041 D2: the tracking home in one read. The day's trips are counted by state, the ten most
-- late are named with their route, destination and minutes, and the day's absences are counted; an
-- organization user's numbers are their organizations' trips only. One SP, refreshed every 30 s by the
-- page: the trips of one day per tenant are tens to a few hundred rows, read through fn_trip_rows and
-- one index probe of trip_stops per open trip.

CREATE FUNCTION tracking.sp_tracking_overview(
    p_tenant_id     UUID,
    p_date          DATE DEFAULT NULL,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    trips_in_progress INT,
    trips_planned     INT,
    arrivals_done     INT,
    absences_today    INT,
    trips_delayed     JSONB,
    has_routes        BOOLEAN
) AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    WITH day AS (
        SELECT t.*
        FROM tracking.fn_trip_rows(p_tenant_id, NULL) t
        WHERE (p_date IS NOT NULL AND t.service_date = p_date
               OR p_date IS NULL
                  AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
                  AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE))
          AND (v_scope IS NULL OR EXISTS (
              SELECT 1
              FROM tracking.trip_stops ts
              INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
              INNER JOIN tracking.riders rd ON rd.id = k.subject_id
              WHERE ts.trip_id = t.id AND k.subject_type = 'passenger' AND rd.organization_id = ANY(v_scope)
          ))
    ),
    late AS (
        -- The board's delay (TRACK-021): the next pending stop overdue by, for a trip still open.
        SELECT d.id, d.route_id, d.route_name,
               CAST(EXTRACT(EPOCH FROM (now() - ns.planned_at)) / 60 AS INT) AS delay_minutes
        FROM day d
        CROSS JOIN LATERAL (
            SELECT ts.planned_at
            FROM tracking.trip_stops ts
            WHERE ts.trip_id = d.id AND ts.status = 'pending'
            ORDER BY ts.sequence
            LIMIT 1
        ) ns
        WHERE d.status IN ('planned', 'in_progress')
          AND ns.planned_at < now() - INTERVAL '5 minutes'
    )
    SELECT
        CAST((SELECT COUNT(*) FROM day d WHERE d.status = 'in_progress') AS INT),
        CAST((SELECT COUNT(*) FROM day d WHERE d.status = 'planned') AS INT),
        CAST((SELECT COUNT(*) FROM day d WHERE d.status = 'completed') AS INT),
        CAST((
            SELECT COUNT(DISTINCT a.rider_id)
            FROM tracking.rider_absences a
            INNER JOIN tracking.riders rd ON rd.id = a.rider_id
            WHERE a.tenant_id = p_tenant_id
              AND COALESCE(p_date, CURRENT_DATE) BETWEEN a.date_from AND a.date_to
              AND (v_scope IS NULL OR rd.organization_id = ANY(v_scope))
        ) AS INT),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_id', l.id, 'route_name', l.route_name, 'organization_name', o.name,
                'delay_minutes', l.delay_minutes) ORDER BY l.delay_minutes DESC)
            FROM (SELECT * FROM late ORDER BY delay_minutes DESC LIMIT 10) l
            INNER JOIN tracking.routes r ON r.id = l.route_id
            LEFT JOIN tracking.organizations o ON o.id = r.organization_id
        ), '[]'::jsonb),
        EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
            WHERE tc.tenant_id = p_tenant_id
        );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_tracking_overview(UUID, DATE, UUID) IS
'The tracking home in one read (TRACK-041 D2): the day''s trips in progress, planned and completed, the riders absent that day, the ten most late trips {trip_id, route_name, organization_name, delay_minutes} and whether the tenant has any route yet; narrowed to p_scope_user_id''s organizations when set';
