CREATE OR REPLACE FUNCTION tracking.fn_route_arrival_minutes(p_tenant_id uuid, p_route_id uuid, p_on date)
 RETURNS integer
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_direction VARCHAR;
    v_minutes   INT;
BEGIN
    SELECT r.direction INTO v_direction
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;
    IF v_direction IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_direction <> 'outbound' THEN
        RAISE EXCEPTION 'route.arrival-outbound-only' USING ERRCODE = 'P0001';
    END IF;

    SELECT tracking.fn_route_version_estimate(p_tenant_id, rv.id) INTO v_minutes
    FROM tracking.route_versions rv
    WHERE rv.route_id = p_route_id
      AND GREATEST(CURRENT_DATE, p_on) >= rv.effective_from
      AND (rv.effective_to IS NULL OR GREATEST(CURRENT_DATE, p_on) <= rv.effective_to);
    RETURN COALESCE(v_minutes, 5);
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_arrival_minutes(uuid,uuid,date) IS 'Minutes an outbound leaves before its arrival, by the estimate of the version in force on p_on; raises route.arrival-outbound-only on a return (TRACK-055 D2)';
