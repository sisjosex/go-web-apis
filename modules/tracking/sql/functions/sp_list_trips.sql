CREATE OR REPLACE FUNCTION tracking.sp_list_trips(p_tenant_id uuid, p_date date DEFAULT NULL::date, p_route_id uuid DEFAULT NULL::uuid, p_status character varying DEFAULT NULL::character varying, p_organization_id uuid DEFAULT NULL::uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_scope_user_id uuid DEFAULT NULL::uuid, p_include_empty boolean DEFAULT true, p_date_to date DEFAULT NULL::date, p_late_min integer DEFAULT NULL::integer)
 RETURNS TABLE(id uuid, route_id uuid, route_name character varying, direction character varying, route_schedule_id uuid, service_date date, timezone character varying, planned_start timestamp with time zone, vehicle_id uuid, license_plate character varying, driver_id uuid, driver_name character varying, status character varying, started_at timestamp with time zone, ended_at timestamp with time zone, is_overridden boolean, stops_count integer, tasks_total integer, tasks_done integer, created_at timestamp with time zone, updated_at timestamp with time zone, delay_seconds integer, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;
    -- TRACK-069 D5: a range starts at p_date and spans at most a month.
    IF p_date_to IS NOT NULL AND (p_date IS NULL OR p_date_to < p_date OR p_date_to - p_date > 31) THEN
        RAISE EXCEPTION 'trip.invalid-range' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT pg.id, pg.route_id, pg.route_name, pg.direction, pg.route_schedule_id, pg.service_date,
           pg.timezone, pg.planned_start, pg.vehicle_id, pg.license_plate, pg.driver_id, pg.driver_name,
           pg.status, pg.started_at, pg.ended_at, pg.is_overridden, pg.stops_count, pg.tasks_total,
           pg.tasks_done, pg.created_at, pg.updated_at,
           -- GREATEST skips a NULL, so the CASE keeps an ended trip's NULL.
           CASE WHEN COALESCE(pg.next_planned_at, ns.planned_at) IS NOT NULL
                THEN CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - COALESCE(pg.next_planned_at, ns.planned_at)))) AS INT) END,
           pg.total_count
    FROM (
    SELECT t.*, lt.planned_at AS next_planned_at, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
    FROM tracking.fn_trip_rows(p_tenant_id, NULL) t
    -- D5: "late only" reads the next pending stop of every candidate before paging, and only then; the
    -- join's condition is a constant, so without p_late_min the subquery never runs.
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = t.id
          AND ts.status = 'pending'
          AND t.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) lt ON p_late_min IS NOT NULL
    WHERE (p_date IS NOT NULL AND t.service_date BETWEEN p_date AND COALESCE(p_date_to, p_date)
           OR p_date IS NULL
              AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
              AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE))
      AND (p_route_id IS NULL OR t.route_id = p_route_id)
      AND (p_status IS NULL OR t.status = p_status)
      -- TRACK-053 D1: a trip nobody rides that day (no task) is left out unless asked for; once
      -- started it always shows — the bus is on the road.
      AND (p_include_empty OR t.tasks_total > 0 OR t.started_at IS NOT NULL)
      -- An organization's trips are the ones carrying one of its riders.
      AND (p_organization_id IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.trip_stops ts
          INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
          INNER JOIN tracking.riders rd ON rd.id = k.subject_id
          WHERE ts.trip_id = t.id
            AND k.subject_type = 'passenger'
            AND rd.organization_id = p_organization_id
      ))
      -- An organization user's trips are the ones carrying a rider of theirs (MOBILE-020).
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1
          FROM tracking.trip_stops ts
          INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
          INNER JOIN tracking.riders rd ON rd.id = k.subject_id
          WHERE ts.trip_id = t.id
            AND k.subject_type = 'passenger'
            AND rd.organization_id = ANY(v_scope)
      ))
      AND (p_late_min IS NULL OR lt.planned_at <= now() - make_interval(mins => p_late_min))
    ORDER BY t.planned_start ASC, t.route_name ASC, t.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size
    ) pg
    -- sp_get_trip_status's next stop, read for the page's rows only — already read above with p_late_min.
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = pg.id
          AND ts.status = 'pending'
          AND pg.status IN ('planned', 'in_progress')
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON p_late_min IS NULL
    ORDER BY pg.planned_start ASC, pg.route_name ASC, pg.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_trips(uuid,date,uuid,character varying,uuid,integer,integer,uuid,boolean,date,integer) IS 'One page of a day''s trips by planned start — p_date (through p_date_to, at most 31 days on, else trip.invalid-range — TRACK-069 D5), or each route''s own today when NULL — narrowed by route, status and an organization whose riders it carries, or p_scope_user_id''s organizations (MOBILE-020); riderless trips not yet started only with p_include_empty (TRACK-053); every row carries delay_seconds (its next pending stop overdue by, NULL once ended — TRACK-021) and total_count (TRACK-020); p_late_min keeps the trips whose next stop is at least that many minutes overdue, filtered before paging (TRACK-069 D5)';
