CREATE OR REPLACE FUNCTION tracking.fn_trip_stops_json(p_tenant_id uuid, p_trip_id uuid)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.fn_trip_stops_json(uuid,uuid) IS 'A trip''s stops by sequence, each with its tasks (pickups first) and the passenger''s name, as the JSONB array the trip detail answers (TRACK-020)';
