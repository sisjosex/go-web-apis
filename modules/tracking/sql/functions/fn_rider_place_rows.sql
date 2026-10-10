CREATE OR REPLACE FUNCTION tracking.fn_rider_place_rows(p_rider_id uuid, p_place_id uuid)
 RETURNS TABLE(id uuid, label character varying, address character varying, latitude numeric, longitude numeric, is_default boolean)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT p.id, CAST(p.label AS VARCHAR), CAST(p.address AS VARCHAR),
           CAST(ST_Y(p.location::geometry) AS DECIMAL), CAST(ST_X(p.location::geometry) AS DECIMAL), p.is_default
    FROM tracking.rider_places p
    WHERE p.rider_id = p_rider_id AND (p_place_id IS NULL OR p.id = p_place_id)
    ORDER BY p.is_default DESC, p.label, p.id;
$function$;
