CREATE OR REPLACE FUNCTION tracking.fn_trip_lock(p_tenant_id uuid, p_trip_id uuid)
 RETURNS tracking.trips
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip tracking.trips%ROWTYPE;
BEGIN
    SELECT t.* INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_trip.id IS NULL THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_trip;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_lock(uuid,uuid) IS 'Returns the tenant''s trip locked FOR UPDATE, so its transitions run one at a time; raises trip.not-found (TRACK-020)';
