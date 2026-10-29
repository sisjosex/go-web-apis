CREATE OR REPLACE FUNCTION tracking.sp_organization_today(p_tenant_id uuid, p_scope_user_id uuid, p_date date DEFAULT NULL::date, p_organization_id uuid DEFAULT NULL::uuid, p_shift time without time zone DEFAULT NULL::time without time zone)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope  UUID[];
    v_riders UUID[];
    v_org    tracking.organizations%ROWTYPE;
    v_date   DATE;
    v_trips  JSONB;
    v_totals JSONB;
    v_shifts JSONB;
BEGIN
    -- A client user reads its own organizations; staff names the one it wants (MOBILE-039 D3).
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    ELSIF p_organization_id IS NULL THEN
        RAISE EXCEPTION 'organization.required' USING ERRCODE = 'P0001';
    END IF;

    SELECT o.* INTO v_org
    FROM tracking.organizations o
    WHERE o.tenant_id = p_tenant_id
      AND (p_organization_id IS NULL OR o.id = p_organization_id)
      AND (v_scope IS NULL OR o.id = ANY(v_scope))
    ORDER BY o.name, o.id
    LIMIT 1;
    IF v_org.id IS NULL THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;
    v_date := COALESCE(p_date, CAST(now() AT TIME ZONE v_org.timezone AS DATE));

    WITH day AS (
        -- The day's trips carrying the client's riders, each with its shift: the local time its last
        -- stop is planned at on a trip in (D2), and when it is expected there now.
        SELECT t.*,
               CASE WHEN t.direction = 'outbound' THEN to_char(t.planned_end AT TIME ZONE t.timezone, 'HH24:MI') END AS shift,
               COALESCE(late.minutes, 0) AS delay_min,
               (SELECT MAX(ts.arrived_at) FROM tracking.trip_stops ts WHERE ts.trip_id = t.id AND t.status = 'completed') AS arrived_at
        FROM tracking.fn_trip_rows(p_tenant_id, ARRAY(
            SELECT DISTINCT tr.id
            FROM tracking.trips tr
            INNER JOIN tracking.trip_stops ts ON ts.trip_id = tr.id
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
            INNER JOIN tracking.riders rd ON rd.id = k.subject_id
            WHERE tr.tenant_id = p_tenant_id
              AND tr.service_date = v_date
              AND tr.status <> 'cancelled'
              AND rd.organization_id = v_org.id
              AND (v_riders IS NULL OR rd.id = ANY(v_riders))
        )) t
        LEFT JOIN LATERAL (
            -- How late the next pending stop is now: the board's delay (TRACK-021), never negative.
            SELECT GREATEST(CAST(EXTRACT(EPOCH FROM (now() - ns.planned_at)) / 60 AS INT), 0) AS minutes
            FROM tracking.trip_stops ns
            WHERE ns.trip_id = t.id AND ns.status = 'pending' AND t.status IN ('planned', 'in_progress')
            ORDER BY ns.sequence
            LIMIT 1
        ) late ON true
    ),
    shifted AS (
        SELECT d.*, d.planned_end + make_interval(mins => d.delay_min) AS expected_arrival,
               -- In the trip's own zone, as its shift is.
               to_char((d.planned_end + make_interval(mins => d.delay_min)) AT TIME ZONE d.timezone, 'HH24:MI') AS expected_local
        FROM day d
        WHERE p_shift IS NULL OR d.shift = to_char(p_shift, 'HH24:MI')
    ),
    riders AS (
        -- One row per rider and trip: where they are now.
        SELECT s.id AS trip_id, k.subject_id AS rider_id, s.expected_local, s.shift,
               CASE
                   WHEN bool_or(k.status = 'cancelled') THEN 'absent'
                   WHEN bool_or(k.kind = 'dropoff' AND k.status = 'done') THEN 'arrived'
                   WHEN bool_or(k.kind = 'pickup' AND k.status = 'done') THEN 'on_the_way'
                   ELSE 'pending'
               END AS state
        FROM shifted s
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = s.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
        INNER JOIN tracking.riders rd ON rd.id = k.subject_id
        WHERE rd.organization_id = v_org.id
          AND (v_riders IS NULL OR rd.id = ANY(v_riders))
        GROUP BY s.id, k.subject_id, s.expected_local, s.shift
    )
    SELECT
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_id', s.id, 'route_id', s.route_id, 'route_name', s.route_name,
                'license_plate', s.license_plate,
                'riders', (SELECT COUNT(*) FROM riders r WHERE r.trip_id = s.id AND r.state <> 'absent'),
                'status', s.status, 'delay_min', s.delay_min, 'expected_arrival', s.expected_arrival,
                'arrived_at', s.arrived_at
            ) ORDER BY s.planned_start, s.id)
            FROM shifted s), '[]'::jsonb),
        jsonb_build_object(
            'expected',   (SELECT COUNT(DISTINCT r.rider_id) FROM riders r),
            'arrived',    (SELECT COUNT(DISTINCT r.rider_id) FROM riders r WHERE r.state = 'arrived'),
            'on_the_way', (SELECT COUNT(DISTINCT r.rider_id) FROM riders r WHERE r.state = 'on_the_way'),
            'absent',     (SELECT COUNT(DISTINCT r.rider_id) FROM riders r WHERE r.state = 'absent'),
            -- Late: riding a trip expected after its shift's hour; on time: every other rider who rides.
            'late',       (SELECT COUNT(DISTINCT r.rider_id) FROM riders r
                           WHERE r.state <> 'absent' AND r.shift IS NOT NULL AND r.expected_local > r.shift),
            'on_time',    (SELECT COUNT(DISTINCT r.rider_id) FROM riders r
                           WHERE r.state <> 'absent' AND (r.shift IS NULL OR r.expected_local <= r.shift))
        ),
        COALESCE((SELECT jsonb_agg(DISTINCT d.shift ORDER BY d.shift) FROM day d WHERE d.shift IS NOT NULL), '[]'::jsonb)
    INTO v_trips, v_totals, v_shifts;

    RETURN jsonb_build_object(
        'organization', jsonb_build_object('id', v_org.id, 'name', v_org.name, 'kind', v_org.kind),
        'organizations', COALESCE((
            SELECT jsonb_agg(jsonb_build_object('id', o.id, 'name', o.name) ORDER BY o.name, o.id)
            FROM tracking.organizations o
            WHERE o.tenant_id = p_tenant_id AND (v_scope IS NULL OR o.id = ANY(v_scope))
              AND (v_scope IS NOT NULL OR o.id = v_org.id)
        ), '[]'::jsonb),
        'date', v_date,
        'shifts', v_shifts,
        'shift', CASE WHEN p_shift IS NOT NULL THEN to_char(p_shift, 'HH24:MI') END,
        'totals', v_totals,
        'routes', v_trips
    );
END;
$function$;

COMMENT ON FUNCTION tracking.sp_organization_today(uuid,uuid,date,uuid,time without time zone) IS 'A school''s or company''s day in one read (MOBILE-039 D3): the organization (the caller''s first when unnamed; staff must name one), the day''s trips carrying its riders with expected arrival (planned last stop + current delay) and arrival, totals per rider (expected, arrived, on the way, absent, late, on time) and the shifts — the local planned arrival of its trips in (D2) — narrowed to p_shift; a supervisor sees its groups only. Raises organization.required, organization.not-found';
