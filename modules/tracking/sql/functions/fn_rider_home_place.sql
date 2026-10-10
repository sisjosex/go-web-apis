CREATE OR REPLACE FUNCTION tracking.fn_rider_home_place(p_rider_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE tracking.rider_places p SET
        location = r.home_location,
        address = COALESCE(r.address, p.address),
        updated_at = CURRENT_TIMESTAMP
    FROM tracking.riders r
    WHERE r.id = p_rider_id AND p.rider_id = r.id AND p.is_default AND r.home_location IS NOT NULL;
    IF NOT FOUND THEN
        INSERT INTO tracking.rider_places (rider_id, label, address, location, is_default)
        SELECT r.id, 'Casa', r.address, r.home_location, true
        FROM tracking.riders r
        WHERE r.id = p_rider_id AND r.home_location IS NOT NULL;
    END IF;
END;
$function$;
