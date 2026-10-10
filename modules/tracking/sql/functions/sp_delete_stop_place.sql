CREATE OR REPLACE FUNCTION tracking.sp_delete_stop_place(p_tenant_id uuid, p_stop_place_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_deleted INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.route_version_stops vs WHERE vs.stop_place_id = p_stop_place_id
    ) THEN
        RAISE EXCEPTION 'stop-place.in-use' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.stop_places sp
    WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted > 0;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_stop_place(uuid,uuid) IS 'Deletes a stop place; raises tracking.stop-place.in-use while any route version still names it';
