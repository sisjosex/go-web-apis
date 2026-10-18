CREATE OR REPLACE FUNCTION tracking.sp_rider_today(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_rider  RECORD;
    v_today  DATE;
    v_trips  JSONB;
BEGIN
    PERFORM tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);

    SELECT r.first_name || ' ' || r.last_name AS name, o.name AS org_name, CAST(o.kind AS VARCHAR) AS org_kind,
           o.timezone
    INTO v_rider
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id;
    v_today := CAST(now() AT TIME ZONE v_rider.timezone AS DATE);

    -- TRACK-061 D2: every trip of each route's local today that carries the rider, morning and
    -- afternoon both, so a leg already ridden stays on the timeline.
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'trip_id',       t.id,
        'route_name',    r.route_name,
        'direction',     r.direction,
        'status',        t.status,
        'planned_start', t.planned_start,
        'started_at',    t.started_at,
        'ended_at',      t.ended_at,
        'license_plate', v.plate_number,
        'driver_name',   d.first_name || ' ' || d.last_name,
        'company_id',    r.company_id,
        'pickup',        legs.stops->'pickup',
        'dropoff',       legs.stops->'dropoff',
        -- Stops the bus still makes before the rider's next one; null once the rider has none left.
        'stops_left',    CASE WHEN legs.next_sequence IS NOT NULL THEN (
            SELECT COUNT(*) FROM tracking.trip_stops o
            WHERE o.trip_id = t.id AND o.status = 'pending' AND o.sequence < legs.next_sequence) END
    ) ORDER BY t.planned_start, t.id), '[]'::jsonb)
    INTO v_trips
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.drivers d ON d.id = t.driver_id
    CROSS JOIN LATERAL (
        SELECT
            jsonb_object_agg(k.kind, jsonb_build_object(
                'trip_stop_id', ts.id,
                'stop_name',    sp.name,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry),
                'planned_at',   ts.planned_at,
                'arrived_at',   ts.arrived_at,
                'task_status',  k.status,
                'done_at',      k.done_at,
                'eta_at',       NULL
            )) AS stops,
            MIN(ts.sequence) FILTER (WHERE ts.status = 'pending' AND k.status = 'pending') AS next_sequence
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
        WHERE ts.trip_id = t.id
          AND k.subject_type = 'passenger'
          AND k.subject_id = p_rider_id
          AND k.status <> 'cancelled'
    ) legs
    WHERE t.tenant_id = p_tenant_id
      AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
      AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
      AND legs.stops IS NOT NULL;

    RETURN jsonb_build_object(
        'rider_id',     p_rider_id,
        'rider_name',   v_rider.name,
        'date',         v_today,
        'timezone',     v_rider.timezone,
        'organization', jsonb_build_object('name', v_rider.org_name, 'kind', v_rider.org_kind),
        -- The carrier of the day's first trip, else of the route the rider is assigned to today.
        'company', (
            SELECT jsonb_build_object('name', c.name, 'phone', c.phone)
            FROM tracking.transport_companies c
            WHERE c.tenant_id = p_tenant_id
              AND c.id = COALESCE(
                  CAST(v_trips->0->>'company_id' AS UUID),
                  (SELECT r.company_id
                   FROM tracking.rider_route_assignments ra
                   INNER JOIN tracking.routes r ON r.id = ra.route_id
                   WHERE ra.rider_id = p_rider_id
                     AND tracking.fn_assignment_on(ra.days_of_week, ra.valid_from, ra.valid_until, v_today)
                   ORDER BY r.direction DESC, r.id
                   LIMIT 1))),
        'absence', (
            SELECT jsonb_build_object('id', a.id, 'direction', a.direction)
            FROM tracking.rider_absences a
            WHERE a.rider_id = p_rider_id
              AND v_today BETWEEN a.date_from AND a.date_to
            ORDER BY a.direction NULLS FIRST, a.created_at
            LIMIT 1),
        'trips', COALESCE((SELECT jsonb_agg(e - 'company_id') FROM jsonb_array_elements(v_trips) e), '[]'::jsonb)
    );
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_today(uuid,uuid,uuid,uuid) IS 'The rider''s day: every trip of each route''s local today that carries them, ordered, each with the rider''s pickup and dropoff (planned, arrived, task status) and the stops left before their next one; the organization, the carrier''s phone and today''s absence; eta_at is left null for the caller; scoped as sp_get_rider_status (TRACK-061 D2)';
