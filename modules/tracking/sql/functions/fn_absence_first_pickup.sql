CREATE OR REPLACE FUNCTION tracking.fn_absence_first_pickup(p_tenant_id uuid, p_rider_id uuid, p_date_from date, p_date_to date, p_direction character varying)
 RETURNS timestamp with time zone
 LANGUAGE sql
 STABLE
AS $function$
    SELECT MIN(COALESCE(ps.planned_at, t.planned_start))
    FROM tracking.rider_route_assignments ra
    INNER JOIN tracking.routes r ON r.id = ra.route_id
    INNER JOIN tracking.trips t
            ON t.route_id = ra.route_id
           AND t.tenant_id = p_tenant_id
           AND t.service_date BETWEEN p_date_from AND p_date_to
           AND t.status IN ('planned', 'in_progress')
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = t.id
        ORDER BY ts.stop_place_id = ra.pickup_stop_place_id DESC NULLS LAST, ts.sequence
        LIMIT 1
    ) ps ON true
    WHERE ra.rider_id = p_rider_id
      AND (p_direction IS NULL OR r.direction = p_direction)
      AND tracking.fn_assignment_on(ra.days_of_week, ra.valid_from, ra.valid_until, t.service_date);
$function$;

COMMENT ON FUNCTION tracking.fn_absence_first_pickup(uuid,uuid,date,date,character varying) IS 'The earliest planned pickup of a rider over [p_date_from, p_date_to] on the not-ended trips of their assigned routes in p_direction (both when NULL): what a guardian''s absence cut-off is measured against (TRACK-022 D1)';
