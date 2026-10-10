CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_route_assignment(p_tenant_id uuid, p_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_old  tracking.rider_route_assignments%ROWTYPE;
    v_twin tracking.rider_route_assignments%ROWTYPE;
    v_on   DATE;
BEGIN
    DELETE FROM tracking.rider_route_assignments a
    USING tracking.riders rd, tracking.organizations o
    WHERE a.id = p_id
      AND rd.id = a.rider_id
      AND o.id = rd.organization_id
      AND o.tenant_id = p_tenant_id
    RETURNING a.* INTO v_old;

    IF v_old.id IS NULL THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    v_on := GREATEST(v_old.valid_from, tracking.fn_route_today(v_old.route_id));
    PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_old.route_id, v_old.rider_id, v_old.pickup_stop_place_id, v_on);
    PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_old.route_id, v_old.rider_id, v_old.dropoff_stop_place_id, v_on);
    PERFORM tracking.fn_route_changed(v_old.route_id, v_on, v_old.valid_until);

    DELETE FROM tracking.rider_route_assignments a
    USING tracking.routes r
    WHERE r.id = v_old.route_id
      AND a.route_id = r.paired_route_id
      AND a.rider_id = v_old.rider_id
      AND a.days_of_week = v_old.days_of_week
      AND a.valid_from = v_old.valid_from
    RETURNING a.* INTO v_twin;
    IF v_twin.id IS NOT NULL THEN
        v_on := GREATEST(v_twin.valid_from, tracking.fn_route_today(v_twin.route_id));
        PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_twin.route_id, v_twin.rider_id, v_twin.pickup_stop_place_id, v_on);
        PERFORM tracking.fn_route_prune_home_stop(p_tenant_id, v_twin.route_id, v_twin.rider_id, v_twin.dropoff_stop_place_id, v_on);
        PERFORM tracking.fn_route_changed(v_twin.route_id, v_on, v_twin.valid_until);
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_rider_route_assignment(uuid,uuid) IS 'Removes one assignment, and its twin on the paired route (TRACK-038 D1), taking their home stops off the routes nobody else calls at them on (TRACK-044 D5), writing route.changed for the days they covered from the route''s today (TRACK-009)';
