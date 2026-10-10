CREATE OR REPLACE FUNCTION tracking.sp_driver_resolve_rider(p_tenant_id uuid, p_user_id uuid, p_token text)
 RETURNS TABLE(rider_id uuid, rider_name text, task_id uuid, trip_id uuid, trip_stop_id uuid, kind character varying, status character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT rd.id, rd.first_name || ' ' || rd.last_name, pk.task_id, pk.trip_id, pk.trip_stop_id, pk.kind, pk.status
    FROM tracking.riders rd
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    LEFT JOIN LATERAL tracking.fn_driver_pending_pickup(p_tenant_id, v_driver.id, rd.id) pk ON true
    WHERE rd.qr_token = upper(regexp_replace(p_token, '\s', '', 'g'))
      AND o.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_resolve_rider(uuid,uuid,text) IS 'The rider a card token names and their pending pickup on the driver account''s trip in progress (task columns NULL when none); raises rider.not-found, driver.not-linked (TRACK-027 D1)';
