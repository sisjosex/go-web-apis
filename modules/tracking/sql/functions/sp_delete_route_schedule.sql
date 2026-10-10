CREATE OR REPLACE FUNCTION tracking.sp_delete_route_schedule(p_tenant_id uuid, p_schedule_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_route_id UUID;
    v_from     DATE;
    v_until    DATE;
    v_deleted  INT;
BEGIN
    SELECT s.route_id, s.valid_from, s.valid_until
      INTO v_route_id, v_from, v_until
    FROM tracking.route_schedules s
    INNER JOIN tracking.routes r ON r.id = s.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE s.id = p_schedule_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'schedule.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_schedules s WHERE s.id = p_schedule_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    PERFORM tracking.fn_route_changed(v_route_id, v_from, v_until);
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_route_schedule(uuid,uuid) IS 'Removes a recurrence from a route and writes one route.changed row over the validity it covered';
