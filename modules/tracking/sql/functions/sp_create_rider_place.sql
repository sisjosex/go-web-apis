CREATE OR REPLACE FUNCTION tracking.sp_create_rider_place(p_tenant_id uuid, p_rider_id uuid, p_label character varying, p_address character varying, p_latitude numeric, p_longitude numeric, p_is_default boolean DEFAULT false, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, label character varying, address character varying, latitude numeric, longitude numeric, is_default boolean)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id      UUID;
    v_default BOOLEAN;
BEGIN
    PERFORM tracking.fn_rider_place_guard(p_tenant_id, p_rider_id, p_scope_user_id);
    PERFORM 1 FROM tracking.riders r WHERE r.id = p_rider_id FOR UPDATE;

    v_default := COALESCE(p_is_default, false)
        OR NOT EXISTS (SELECT 1 FROM tracking.rider_places p WHERE p.rider_id = p_rider_id);
    IF v_default THEN
        UPDATE tracking.rider_places p SET is_default = false, updated_at = CURRENT_TIMESTAMP
        WHERE p.rider_id = p_rider_id AND p.is_default;
    END IF;

    INSERT INTO tracking.rider_places (rider_id, label, address, location, is_default)
    VALUES (p_rider_id, TRIM(p_label), NULLIF(TRIM(p_address), ''),
            ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography, v_default)
    RETURNING tracking.rider_places.id INTO v_id;

    IF v_default THEN
        PERFORM tracking.fn_rider_place_mirror(p_rider_id);
    END IF;
    RETURN QUERY SELECT * FROM tracking.fn_rider_place_rows(p_rider_id, v_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_rider_place(uuid,uuid,character varying,character varying,numeric,numeric,boolean,uuid) IS 'Saves a place for a rider; the first one, or one sent as default, becomes the rider''s home (TRACK-044 D4)';
