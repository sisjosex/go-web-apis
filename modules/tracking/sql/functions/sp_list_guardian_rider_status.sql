CREATE OR REPLACE FUNCTION tracking.sp_list_guardian_rider_status(p_tenant_id uuid, p_guardian_user_id uuid)
 RETURNS TABLE(rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, vehicle_id uuid, license_plate character varying, driver_name character varying, driver_id uuid, vehicle_latitude numeric, vehicle_longitude numeric, vehicle_speed numeric, location_age_seconds integer, last_event_type character varying, last_event_time timestamp without time zone, last_event_notes text, last_event_stop character varying, scheduled_pickup_stop character varying, scheduled_dropoff_stop character varying, active_alerts integer, last_event_task_id uuid, last_event_has_photo boolean, organization_kind character varying, rider_type character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_riders UUID[] := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
BEGIN
    -- Each of the guardian's riders through the one status rule there is: a guardian has 1–5 of them,
    -- so this is k calls in one round trip instead of k round trips (MOBILE-026 D1). The client's kind
    -- and the rider's type let a home that mixes operators tell a pupil from an employee (TRACK-061).
    -- The riders already come from the guardian's scope, so each status is read unscoped: the home is
    -- logged once as 'today', not once more per rider as 'status' (TRACK-063).
    RETURN QUERY
    SELECT s.*, CAST(o.kind AS VARCHAR), CAST(r.rider_type AS VARCHAR)
    FROM unnest(v_riders) AS g(id)
    CROSS JOIN LATERAL tracking.sp_get_rider_status(p_tenant_id, g.id, NULL, NULL) AS s
    INNER JOIN tracking.riders r ON r.id = g.id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    ORDER BY s.rider_name, s.rider_id;

    PERFORM tracking.fn_record_rider_access(p_tenant_id, p_guardian_user_id, 'portal', 'today', v_riders);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_guardian_rider_status(uuid,uuid) IS 'Today''s status of every rider p_guardian_user_id is a guardian of, as sp_get_rider_status answers it for one, plus the client''s kind and the rider''s type (MOBILE-026 D1, TRACK-061): the guardian home in one call, logged as a ''today'' read of each rider (TRACK-063)';
