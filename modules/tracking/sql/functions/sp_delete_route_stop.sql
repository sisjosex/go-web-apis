CREATE OR REPLACE FUNCTION tracking.sp_delete_route_stop(p_stop_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if stop exists
    IF NOT EXISTS (SELECT 1 FROM tracking.route_stops WHERE id = p_stop_id) THEN
        RAISE EXCEPTION 'route-stop.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete by setting status to inactive
    UPDATE tracking.route_stops
    SET status = 'inactive',
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_stop_id;
    
    RETURN TRUE;
END;
$function$;
