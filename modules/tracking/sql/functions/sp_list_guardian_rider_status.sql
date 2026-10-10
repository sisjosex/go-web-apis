CREATE OR REPLACE FUNCTION tracking.sp_list_guardian_rider_status(p_tenant_id uuid, p_guardian_user_id uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer)
 LANGUAGE sql
AS $function$
    -- Each of the guardian's riders through the one status rule there is: a guardian has 1–5 of them,
    -- so this is k calls in one round trip instead of k round trips (MOBILE-026 D1).
    SELECT s.*
    FROM unnest(tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id)) AS g(id)
    CROSS JOIN LATERAL tracking.sp_get_rider_status(p_tenant_id, g.id, NULL, p_guardian_user_id) AS s
    ORDER BY s.rider_name, s.rider_id;
$function$;

COMMENT ON FUNCTION tracking.sp_list_guardian_rider_status(uuid,uuid) IS 'Today''s status of every rider p_guardian_user_id is a guardian of, as sp_get_rider_status answers it for one (MOBILE-026 D1): the guardian home in one call';
