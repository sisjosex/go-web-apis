CREATE OR REPLACE FUNCTION tracking.fn_driver_pending_pickup(p_tenant_id uuid, p_driver_id uuid, p_rider_id uuid)
 RETURNS TABLE(task_id uuid, trip_id uuid, trip_stop_id uuid, kind character varying, status character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT k.id, t.id, ts.id, CAST(k.kind AS VARCHAR), CAST(k.status AS VARCHAR)
    FROM tracking.trips t
    INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
    WHERE t.tenant_id = p_tenant_id
      AND t.driver_id = p_driver_id
      AND t.status = 'in_progress'
      AND k.subject_type = 'passenger'
      AND k.subject_id = p_rider_id
      AND k.kind = 'pickup'
      AND k.status = 'pending'
    ORDER BY t.started_at, ts.sequence
    LIMIT 1;
$function$;

COMMENT ON FUNCTION tracking.fn_driver_pending_pickup(uuid,uuid,uuid) IS 'A rider''s pending pickup on a driver''s trips in progress, earliest stop first (TRACK-027)';
