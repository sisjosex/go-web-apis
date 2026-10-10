CREATE OR REPLACE FUNCTION tracking.sp_list_rider_places(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, label character varying, address character varying, latitude numeric, longitude numeric, is_default boolean)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    PERFORM tracking.fn_rider_place_guard(p_tenant_id, p_rider_id, p_scope_user_id);
    RETURN QUERY SELECT * FROM tracking.fn_rider_place_rows(p_rider_id, NULL);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_places(uuid,uuid,uuid) IS 'A rider''s saved places, the default first; raises rider.not-found outside the tenant or scope (TRACK-044 D4)';
