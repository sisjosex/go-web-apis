CREATE OR REPLACE FUNCTION tracking.fn_rider_place_mirror(p_rider_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_place tracking.rider_places%ROWTYPE;
BEGIN
    SELECT p.* INTO v_place FROM tracking.rider_places p WHERE p.rider_id = p_rider_id AND p.is_default;
    UPDATE tracking.riders r SET
        home_location = v_place.location,
        -- A pin a person moved is no longer the import's guess (TRACK-072 D3).
        home_geocoded = r.home_geocoded AND v_place.location IS NOT NULL AND r.home_location IS NOT NULL
            AND ST_Equals(r.home_location::geometry, v_place.location::geometry),
        address = CASE WHEN v_place.id IS NULL THEN r.address ELSE v_place.address END,
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;
END;
$function$;
