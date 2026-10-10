CREATE OR REPLACE FUNCTION tracking.sp_delete_route_exception(p_tenant_id uuid, p_exception_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_route_id UUID;
    v_from     DATE;
    v_to       DATE;
    v_deleted  INT;
BEGIN
    SELECT e.route_id, e.date_from, e.date_to
      INTO v_route_id, v_from, v_to
    FROM tracking.route_exceptions e
    INNER JOIN tracking.routes r ON r.id = e.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE e.id = p_exception_id AND tc.tenant_id = p_tenant_id;

    IF v_route_id IS NULL THEN
        RAISE EXCEPTION 'exception.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.route_exceptions e WHERE e.id = p_exception_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;

    -- Taking an exception back changes the same days it changed when it was made.
    PERFORM tracking.fn_route_changed(v_route_id, v_from, v_to);
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_route_exception(uuid,uuid) IS 'Removes an exception and writes one route.changed row over the days it covered';
