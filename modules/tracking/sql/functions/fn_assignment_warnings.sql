CREATE OR REPLACE FUNCTION tracking.fn_assignment_warnings(p_tenant_id uuid, p_route_ids uuid[])
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    WITH win AS (
        SELECT w.route_id, w.date_from, w.date_to
        FROM (SELECT DISTINCT unnest(p_route_ids) AS route_id) ids
        CROSS JOIN LATERAL tracking.fn_trip_window(p_tenant_id, ids.route_id, NULL, NULL) w
    ),
    plan AS (
        SELECT DISTINCT w.route_id, p.service_date, p.vehicle_id
        FROM win w
        CROSS JOIN LATERAL tracking.sp_preview_route(p_tenant_id, w.route_id, w.date_from, w.date_to) p
        WHERE p.status = 'planned'
    ),
    load AS (
        SELECT w.route_id, CAST(g AS DATE) AS service_date, COUNT(*) AS assigned
        FROM win w
        CROSS JOIN generate_series(w.date_from, w.date_to, INTERVAL '1 day') g
        INNER JOIN tracking.rider_route_assignments a
                ON a.route_id = w.route_id
               AND tracking.fn_assignment_on(a.days_of_week, a.valid_from, a.valid_until, CAST(g AS DATE))
        GROUP BY w.route_id, CAST(g AS DATE)
    )
    SELECT COALESCE(jsonb_agg(jsonb_build_object(
        'service_date', pl.service_date,
        'route_id',     pl.route_id,
        'vehicle_id',   pl.vehicle_id,
        'capacity',     COALESCE(r.capacity, v.capacity),
        'assigned',     l.assigned
    ) ORDER BY pl.service_date, pl.route_id, pl.vehicle_id), '[]'::jsonb)
    FROM plan pl
    INNER JOIN load l ON l.route_id = pl.route_id AND l.service_date = pl.service_date
    INNER JOIN tracking.routes r ON r.id = pl.route_id
    LEFT JOIN tracking.vehicles v ON v.id = pl.vehicle_id
    WHERE l.assigned > COALESCE(r.capacity, v.capacity);
$function$;

COMMENT ON FUNCTION tracking.fn_assignment_warnings(uuid,uuid[]) IS 'The days of each route''s materialised window on which a planned vehicle seats fewer riders than are assigned, as [{service_date, route_id, vehicle_id, capacity, assigned}] (TRACK-009 D3)';
