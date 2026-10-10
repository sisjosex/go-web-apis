CREATE OR REPLACE FUNCTION tracking.sp_driver_find_riders(p_tenant_id uuid, p_user_id uuid, p_query text)
 RETURNS TABLE(rider_id uuid, rider_name text, task_id uuid, trip_id uuid, trip_stop_id uuid, kind character varying, status character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_driver  tracking.drivers%ROWTYPE;
    v_pattern TEXT := '%' || replace(replace(replace(btrim(p_query), '\', '\\'), '%', '\%'), '_', '\_') || '%';
    v_code    TEXT := upper(regexp_replace(p_query, '\s', '', 'g'));
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT rd.id, rd.first_name || ' ' || rd.last_name, pk.task_id, pk.trip_id, pk.trip_stop_id, pk.kind, pk.status
    FROM tracking.riders rd
    LEFT JOIN LATERAL tracking.fn_driver_pending_pickup(p_tenant_id, v_driver.id, rd.id) pk ON true
    WHERE rd.id IN (
        SELECT k.subject_id
        FROM tracking.trips t
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE t.tenant_id = p_tenant_id
          AND t.driver_id = v_driver.id
          AND t.status = 'in_progress'
          AND k.subject_type = 'passenger'
    )
      AND ((rd.first_name || ' ' || rd.last_name) ILIKE v_pattern OR rd.qr_token = v_code)
    ORDER BY rd.last_name, rd.first_name, rd.id
    LIMIT 20;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_driver_find_riders(uuid,uuid,text) IS 'Riders on the driver account''s trips in progress whose name contains p_query or whose card code is p_query, each with their pending pickup; at most 20; raises driver.not-linked (TRACK-027 D1)';
