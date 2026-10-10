CREATE OR REPLACE FUNCTION tracking.sp_get_vehicle_current_location(p_vehicle_id uuid)
 RETURNS TABLE(vehicle_id uuid, plate_number character varying, latitude numeric, longitude numeric, speed numeric, heading numeric, recorded_at timestamp without time zone, age_seconds integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if vehicle exists
    IF NOT EXISTS (SELECT 1 FROM tracking.vehicles v WHERE v.id = p_vehicle_id) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        v.id,
        v.plate_number,
        vl.latitude,
        vl.longitude,
        vl.speed,
        vl.heading,
        CAST(vl.recorded_at AS TIMESTAMP),
        CASE 
            WHEN vl.recorded_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at))::INT 
            ELSE NULL 
        END AS age_seconds
    FROM tracking.vehicles v
    LEFT JOIN LATERAL (
        SELECT CAST(ST_Y(lp.location::geometry) AS NUMERIC) AS latitude,
               CAST(ST_X(lp.location::geometry) AS NUMERIC) AS longitude,
               CAST(lp.speed AS NUMERIC) AS speed,
               CAST(lp.heading AS NUMERIC) AS heading,
               lp.recorded_at
        FROM tracking.vehicle_last_position lp
        WHERE lp.vehicle_id = p_vehicle_id
    ) vl ON true
    WHERE v.id = p_vehicle_id;
END;
$function$;
