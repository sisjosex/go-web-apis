CREATE OR REPLACE FUNCTION tracking.sp_update_rider_place(p_tenant_id uuid, p_rider_id uuid, p_place_id uuid, p_label character varying DEFAULT NULL::character varying, p_address character varying DEFAULT NULL::character varying, p_latitude numeric DEFAULT NULL::numeric, p_longitude numeric DEFAULT NULL::numeric, p_is_default boolean DEFAULT NULL::boolean, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, label character varying, address character varying, latitude numeric, longitude numeric, is_default boolean)
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

    IF COALESCE(p_is_default, false) AND NOT v_default THEN
        UPDATE tracking.rider_places p SET is_default = false, updated_at = CURRENT_TIMESTAMP
        WHERE p.rider_id = p_rider_id AND p.is_default;
        v_default := true;
    END IF;

    UPDATE tracking.rider_places p SET
        label = COALESCE(NULLIF(TRIM(p_label), ''), p.label),
        address = CASE WHEN p_address IS NULL THEN p.address ELSE NULLIF(TRIM(p_address), '') END,
        location = CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography ELSE p.location END,
        is_default = v_default,
        updated_at = CURRENT_TIMESTAMP
    WHERE p.id = p_place_id;

    IF v_default THEN
        PERFORM tracking.fn_rider_place_mirror(p_rider_id);
    END IF;
    RETURN QUERY SELECT * FROM tracking.fn_rider_place_rows(p_rider_id, p_place_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_rider_place(uuid,uuid,uuid,character varying,character varying,numeric,numeric,boolean,uuid) IS 'Edits a rider''s place, NULL keeps a field; p_is_default makes it the rider''s home; raises rider-place.not-found (TRACK-044 D4)';
