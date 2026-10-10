CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_plate(p_vehicle_id uuid)
 RETURNS TABLE(plate_number character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate vehicle exists and return plate number
    RETURN QUERY
    SELECT CAST(v.plate_number AS VARCHAR)
    FROM tracking.vehicles v
    WHERE v.id = p_vehicle_id;

    -- If no results, raise exception
    IF NOT FOUND THEN
        RAISE EXCEPTION 'tracking.vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;
