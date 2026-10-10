CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_place(p_tenant_id uuid, p_rider_id uuid, p_place_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_default BOOLEAN;
BEGIN
    PERFORM tracking.fn_rider_place_guard(p_tenant_id, p_rider_id, p_scope_user_id);
    PERFORM 1 FROM tracking.riders r WHERE r.id = p_rider_id FOR UPDATE;

    SELECT p.is_default INTO v_default
    FROM tracking.rider_places p WHERE p.id = p_place_id AND p.rider_id = p_rider_id;
    IF v_default IS NULL THEN
        RAISE EXCEPTION 'rider-place.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_default AND EXISTS (
        SELECT 1 FROM tracking.rider_places p WHERE p.rider_id = p_rider_id AND p.id <> p_place_id
    ) THEN
        RAISE EXCEPTION 'rider-place.default' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_places p WHERE p.id = p_place_id;
    IF v_default THEN
        PERFORM tracking.fn_rider_place_mirror(p_rider_id);
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_rider_place(uuid,uuid,uuid,uuid) IS 'Deletes a rider''s place; the default only when it is the last one, which clears the home pin; raises rider-place.not-found, rider-place.default (TRACK-044 D4)';
