CREATE OR REPLACE FUNCTION tracking.sp_share_trip_public(p_tenant_id uuid, p_trip_id uuid, p_rider_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(scope character varying, status character varying, route_name character varying, license_plate character varying, direction character varying, service_date date, timezone character varying, planned_start timestamp with time zone, started_at timestamp with time zone, ended_at timestamp with time zone, rider_first_name character varying, stops jsonb, vehicle_position jsonb, planned_path jsonb, destination jsonb)
 LANGUAGE sql
 STABLE
AS $function$
    -- D1: no driver and no passenger names; a guardian's link (p_rider_id) shows that rider's stops
    -- and first name only, and the bus until the rider's dropoff is done (TRACK-030 D1).
    SELECT
        CAST(CASE WHEN p_rider_id IS NULL THEN 'trip' ELSE 'rider' END AS VARCHAR),
        CAST(t.status AS VARCHAR),
        CAST(r.route_name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        CAST(r.direction AS VARCHAR),
        t.service_date,
        CAST(t.timezone AS VARCHAR),
        t.planned_start,
        t.started_at,
        t.ended_at,
        CAST(rd.first_name AS VARCHAR),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'sequence',     ts.sequence,
                'stop_name',    sp.name,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry),
                'planned_at',   ts.planned_at,
                'arrived_at',   ts.arrived_at,
                'status',       ts.status
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id
              AND (p_rider_id IS NULL OR EXISTS (
                    SELECT 1 FROM tracking.trip_stop_tasks k
                    WHERE k.trip_stop_id = ts.id
                      AND k.subject_type = 'passenger'
                      AND k.subject_id = p_rider_id
                      AND k.status <> 'cancelled'))
        ), '[]'::jsonb),
        CASE WHEN t.status = 'in_progress' AND lp.vehicle_id IS NOT NULL
              AND (p_rider_id IS NULL OR NOT EXISTS (
                    SELECT 1
                    FROM tracking.trip_stops ts
                    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
                    WHERE ts.trip_id = t.id
                      AND k.subject_type = 'passenger'
                      AND k.subject_id = p_rider_id
                      AND k.kind = 'dropoff'
                      AND k.status IN ('done', 'no_show')))
        THEN jsonb_build_object(
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'heading',     lp.heading,
            'speed',       lp.speed,
            'recorded_at', lp.recorded_at
        ) END,
        tracking.fn_trip_planned_path(t.route_version_id),
        tracking.fn_trip_destination(p_tenant_id, t.id)
    FROM tracking.trips t
    INNER JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = t.vehicle_id
    LEFT JOIN tracking.riders rd ON rd.id = p_rider_id
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_share_trip_public(uuid,uuid,uuid) IS 'A shared trip as the public page draws it: header, stops with planned and actual times, the bus while it runs and the planned line, never a driver''s or passenger''s name; with p_rider_id only that rider''s stops, first name, and the bus until their dropoff is done (TRACK-060 D1)';
