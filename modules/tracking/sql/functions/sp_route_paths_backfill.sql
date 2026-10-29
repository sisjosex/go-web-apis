CREATE OR REPLACE FUNCTION tracking.sp_route_paths_backfill(p_tenant_id uuid)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_enqueued INT;
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'route.changed', jsonb_build_object('route_id', p.route_id, 'date_from', NULL, 'date_to', NULL)
    FROM (
        SELECT DISTINCT rv.route_id
        FROM tracking.route_versions rv
        INNER JOIN tracking.routes r ON r.id = rv.route_id
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE (rv.path_computed_at IS NULL
               -- A street line stored before TRACK-071 has no geography for the off-route check yet.
               OR (rv.path_source = 'valhalla' AND rv.planned_polyline IS NOT NULL AND rv.planned_line IS NULL))
          AND tc.tenant_id = p_tenant_id
    ) p;
    GET DIAGNOSTICS v_enqueued = ROW_COUNT;
    RETURN v_enqueued;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_route_paths_backfill(uuid) IS 'Writes one route.changed outbox row per route of the tenant with a version whose planned line was never computed, or whose street line has no planned_line geography yet; answers how many (TRACK-028, TRACK-071)';
