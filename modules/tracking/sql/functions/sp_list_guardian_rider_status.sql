CREATE OR REPLACE FUNCTION tracking.sp_list_guardian_rider_status(p_tenant_id uuid, p_guardian_user_id uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, driver_id uuid, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer, last_event_task_id uuid, last_event_has_photo boolean, organization_kind character varying, rider_type character varying, organization_name character varying, absence_id uuid, absence_direction character varying, absence_created_at timestamp without time zone, live_trip_id uuid, live_stop_id uuid, qr_token character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_riders UUID[] := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
BEGIN
    -- Each of the guardian's riders through the one status rule there is: a guardian has 1–5 of them,
    -- so this is k calls in one round trip instead of k round trips (MOBILE-026 D1). The client's kind
    -- and the rider's type let a home that mixes operators tell a pupil from an employee (TRACK-061).
    -- The riders already come from the guardian's scope, so each status is read unscoped: the home is
    -- logged once as 'today', not once more per rider as 'status' (TRACK-063).
    -- MOBILE-038 D1: the client's name and today's absence (both legs first, then the earliest) for the
    -- home's cards, and the trip in progress carrying the rider with the stop they wait for next — the
    -- pickup before boarding, the drop-off after — whose ETA the caller reads from the shared estimate.
    RETURN QUERY
    SELECT s.*, CAST(o.kind AS VARCHAR), CAST(r.rider_type AS VARCHAR), CAST(o.name AS VARCHAR),
           ab.id, CAST(ab.direction AS VARCHAR), ab.created_at, lv.trip_id, lv.stop_id,
           -- MOBILE-039 D1: the pass, to the passenger's own account only — never to a guardian.
           CASE WHEN EXISTS (
               SELECT 1 FROM tracking.rider_contacts sc
               WHERE sc.rider_id = g.id AND sc.user_id = p_guardian_user_id AND sc.relation = 'self'
           ) THEN CAST(r.qr_token AS VARCHAR) END
    FROM unnest(v_riders) AS g(id)
    CROSS JOIN LATERAL tracking.sp_get_rider_status(p_tenant_id, g.id, NULL, NULL) AS s
    INNER JOIN tracking.riders r ON r.id = g.id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN LATERAL (
        SELECT a.id, a.direction, a.created_at
        FROM tracking.rider_absences a
        WHERE a.rider_id = g.id
          AND CAST(now() AT TIME ZONE o.timezone AS DATE) BETWEEN a.date_from AND a.date_to
        ORDER BY a.direction NULLS FIRST, a.created_at
        LIMIT 1
    ) ab ON true
    LEFT JOIN LATERAL (
        SELECT t.id AS trip_id, ts.id AS stop_id
        FROM tracking.trips t
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE t.tenant_id = p_tenant_id
          AND t.status = 'in_progress'
          AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
          AND k.subject_type = 'passenger'
          AND k.subject_id = g.id
          AND k.status = 'pending'
          -- Not after a no-show: a rider who never boarded is waited for nowhere.
          AND NOT EXISTS (
              SELECT 1 FROM tracking.trip_stops ns
              INNER JOIN tracking.trip_stop_tasks nk ON nk.trip_stop_id = ns.id
              WHERE ns.trip_id = t.id AND nk.subject_type = 'passenger' AND nk.subject_id = g.id
                AND nk.status IN ('no_show', 'cancelled') AND nk.kind = 'pickup')
        ORDER BY ts.sequence
        LIMIT 1
    ) lv ON true
    ORDER BY s.rider_name, s.rider_id;

    PERFORM tracking.fn_record_rider_access(p_tenant_id, p_guardian_user_id, 'portal', 'today', v_riders);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_guardian_rider_status(uuid,uuid) IS 'Today''s status of every rider p_guardian_user_id is a guardian of, as sp_get_rider_status answers it for one, plus the client''s kind, name and the rider''s type, today''s absence, the in-progress trip with the rider''s next stop for the ETA, and the rider''s card code to a self contact only (MOBILE-026 D1, TRACK-061, MOBILE-038 D1, MOBILE-039 D1): the guardian home in one call, logged as a ''today'' read of each rider (TRACK-063)';
