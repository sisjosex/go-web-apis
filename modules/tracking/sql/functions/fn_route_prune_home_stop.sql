CREATE OR REPLACE FUNCTION tracking.fn_route_prune_home_stop(p_tenant_id uuid, p_route_id uuid, p_rider_id uuid, p_stop_place_id uuid, p_on date)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_version RECORD;
    v_target  UUID;
    v_at      INT;
    v_removed BOOLEAN := false;
BEGIN
    IF p_stop_place_id IS NULL OR NOT EXISTS (
        SELECT 1
        FROM tracking.riders rd
        INNER JOIN tracking.stop_places sp ON sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id
        INNER JOIN tracking.routes r ON r.id = p_route_id
        LEFT JOIN tracking.organizations o ON o.id = r.organization_id AND o.tenant_id = p_tenant_id
        WHERE rd.id = p_rider_id
          AND ST_DWithin(sp.location, rd.home_location, 30)
          AND (o.location IS NULL OR NOT ST_DWithin(sp.location, o.location, 30))
    ) OR EXISTS (
        SELECT 1 FROM tracking.rider_route_assignments a
        WHERE a.route_id = p_route_id
          AND p_stop_place_id IN (a.pickup_stop_place_id, a.dropoff_stop_place_id)
          AND (a.valid_until IS NULL OR a.valid_until >= p_on)
    ) THEN
        RETURN;
    END IF;

    FOR v_version IN
        SELECT rv.* FROM tracking.route_versions rv
        WHERE rv.route_id = p_route_id
          AND (rv.effective_to IS NULL OR rv.effective_to >= p_on)
          AND EXISTS (
              SELECT 1 FROM tracking.route_version_stops vs
              WHERE vs.version_id = rv.id AND vs.stop_place_id = p_stop_place_id)
        ORDER BY rv.effective_from
    LOOP
        v_target := tracking.fn_route_version_split(v_version.id, p_on);
        DELETE FROM tracking.route_version_stops vs
        WHERE vs.version_id = v_target AND vs.stop_place_id = p_stop_place_id
        RETURNING vs.sequence INTO v_at;
        -- Close the gap in two passes, as fn_route_home_stop opens one.
        UPDATE tracking.route_version_stops vs SET sequence = vs.sequence + 100000
        WHERE vs.version_id = v_target AND vs.sequence > v_at;
        UPDATE tracking.route_version_stops vs SET sequence = vs.sequence - 100001, updated_at = CURRENT_TIMESTAMP
        WHERE vs.version_id = v_target AND vs.sequence > 100000;
        v_removed := true;
    END LOOP;

    IF v_removed THEN
        PERFORM tracking.fn_route_recompute_departure(p_tenant_id, p_route_id, p_on);
        PERFORM tracking.fn_route_changed(p_route_id, p_on, NULL);
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_prune_home_stop(uuid,uuid,uuid,uuid,date) IS 'Takes a rider''s home stop off the route''s versions in force from p_on once no assignment of the route calls there, and moves the auto departure (TRACK-044 D5)';
