-- MOBILE-015 step 1 (D2): each trip of the driver's day carries the planned line of its route version
-- (TRACK-028), so the phone draws the route and measures what is left without a request or a signal.
-- ~1-3 KB a trip, read in the same statement through trips.route_version_id; storing a line changes
-- the day, hence its ETag.

DROP FUNCTION IF EXISTS tracking.sp_driver_today(UUID, UUID);

-- The driver's trips of each route's local today, any status, in the list's row shape, each with its
-- stops → tasks → rider. It is what the phone keeps offline, so it carries what a stop and a rider
-- need on the road (coordinates, the rider's notes and card token, the planned line) and nothing the
-- web detail adds.
-- The BETWEEN keeps the (tenant_id, service_date, status) index in play before the per-zone test, as
-- sp_list_trips does.
CREATE FUNCTION tracking.sp_driver_today(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS JSONB AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trips  JSONB;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT COALESCE(jsonb_agg(to_jsonb(r) || jsonb_build_object('stops', st.stops, 'planned_path', pp.path) ORDER BY r.planned_start, r.id), '[]'::jsonb)
    INTO v_trips
    FROM tracking.fn_trip_rows(p_tenant_id, ARRAY(
        SELECT t.id
        FROM tracking.trips t
        WHERE t.tenant_id = p_tenant_id
          AND t.driver_id = v_driver.id
          AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
          AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
    )) r
    CROSS JOIN LATERAL (
        SELECT COALESCE(jsonb_agg(jsonb_build_object(
            'id',         ts.id,
            'sequence',   ts.sequence,
            'stop_name',  sp.name,
            'lat',        ST_Y(sp.location::geometry),
            'lng',        ST_X(sp.location::geometry),
            'planned_at', ts.planned_at,
            'arrived_at', ts.arrived_at,
            'status',     ts.status,
            'tasks',      tk.tasks
        ) ORDER BY ts.sequence), '[]'::jsonb) AS stops
        FROM tracking.trip_stops ts
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        CROSS JOIN LATERAL (
            SELECT COALESCE(jsonb_agg(jsonb_build_object(
                'id',      k.id,
                'kind',    k.kind,
                'status',  k.status,
                'done_at', k.done_at,
                'rider',   CASE WHEN rd.id IS NOT NULL THEN jsonb_build_object(
                    'id',       rd.id,
                    'name',     rd.first_name || ' ' || rd.last_name,
                    'notes',    rd.notes,
                    'qr_token', rd.qr_token
                ) END
            ) ORDER BY k.kind DESC, rd.last_name, rd.first_name, k.id), '[]'::jsonb) AS tasks
            FROM tracking.trip_stop_tasks k
            LEFT JOIN tracking.riders rd ON k.subject_type = 'passenger' AND rd.id = k.subject_id
            WHERE k.trip_stop_id = ts.id
        ) tk
        WHERE ts.trip_id = r.id
    ) st
    LEFT JOIN LATERAL (
        SELECT jsonb_build_object(
            'polyline6',  rv.planned_polyline,
            'distance_m', rv.planned_distance_m,
            'legs',       COALESCE(rv.planned_legs, '[]'::jsonb),
            'source',     rv.path_source
        ) AS path
        FROM tracking.trips t
        INNER JOIN tracking.route_versions rv ON rv.id = t.route_version_id
        WHERE t.id = r.id
          AND rv.planned_polyline IS NOT NULL
    ) pp ON true;

    RETURN jsonb_build_object(
        'date',   COALESCE(
            (SELECT MIN(CAST(e->>'service_date' AS DATE)) FROM jsonb_array_elements(v_trips) e),
            CURRENT_DATE),
        'driver', jsonb_build_object('id', v_driver.id, 'name', v_driver.first_name || ' ' || v_driver.last_name),
        'trips',  v_trips
    );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_driver_today(UUID, UUID) IS
'The driver account''s trips of each route''s local today, any status: { date, driver { id, name }, trips [row + stops [{ id, sequence, stop_name, lat, lng, planned_at, arrived_at, status, tasks [{ id, kind, status, done_at, rider { id, name, notes, qr_token } }] }], planned_path { polyline6, distance_m, legs [{ distance_m, duration_s }], source } | null] }; raises driver.not-linked (TRACK-011, MOBILE-006 D2, MOBILE-015 D2)';
