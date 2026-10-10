CREATE OR REPLACE FUNCTION tracking.sp_update_rider_route_assignment(p_tenant_id uuid, p_id uuid, p_changes jsonb)
 RETURNS TABLE(id uuid, rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, direction character varying, days_of_week smallint, pickup_stop_place_id uuid, pickup_stop_name character varying, dropoff_stop_place_id uuid, dropoff_stop_name character varying, valid_from date, valid_until date, created_at timestamp without time zone, updated_at timestamp without time zone, warnings jsonb)
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM tracking.fn_assignment_put(p_tenant_id, p_id, COALESCE(p_changes, '{}'::jsonb) - 'rider_id');

    RETURN QUERY
    SELECT a.*, tracking.fn_assignment_warnings(p_tenant_id, ARRAY[a.route_id])
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, ARRAY[p_id]) a;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_rider_route_assignment(uuid,uuid,jsonb) IS 'Edits one assignment — every key of p_changes but rider_id, a key left out keeps its value — and returns it with its route''s capacity warnings (TRACK-009)';
