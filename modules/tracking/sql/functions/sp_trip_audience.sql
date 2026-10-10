CREATE OR REPLACE FUNCTION tracking.sp_trip_audience(p_tenant_id uuid, p_trip_id uuid, p_subject_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_ids uuid[], organization_ids uuid[])
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        COALESCE(array_agg(DISTINCT k.subject_id) FILTER (WHERE k.id = p_subject_id OR ts.id = p_subject_id OR NOT EXISTS (
            SELECT 1
            FROM tracking.trip_stop_tasks f
            INNER JOIN tracking.trip_stops fs ON fs.id = f.trip_stop_id
            WHERE fs.trip_id = t.id
              AND f.subject_type = 'passenger'
              AND f.subject_id = k.subject_id
              AND ((f.kind = 'dropoff' AND f.status IN ('done', 'no_show'))
                OR (f.kind = 'pickup' AND f.status = 'no_show')))), '{}'::UUID[]),
        COALESCE(array_agg(DISTINCT rd.organization_id) FILTER (WHERE rd.organization_id IS NOT NULL), '{}'::UUID[])
    FROM tracking.trips t
    INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.subject_type = 'passenger'
    LEFT JOIN tracking.riders rd ON rd.id = k.subject_id
    WHERE t.id = p_trip_id
      AND t.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_audience(uuid,uuid,uuid) IS 'The riders of a trip not yet off it — plus those with a task on p_subject_id, the task or stop the transition moved — and every rider''s organization: the rider and organization-fleet channels its changes are published to (TRACK-025, TRACK-030)';
