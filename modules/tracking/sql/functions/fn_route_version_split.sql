CREATE OR REPLACE FUNCTION tracking.fn_route_version_split(p_version_id uuid, p_on date)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_version tracking.route_versions%ROWTYPE;
    v_target  UUID;
BEGIN
    SELECT rv.* INTO v_version FROM tracking.route_versions rv WHERE rv.id = p_version_id;
    IF v_version.effective_from >= p_on THEN
        RETURN p_version_id;
    END IF;

    UPDATE tracking.route_versions rv
       SET effective_to = p_on - 1, updated_at = CURRENT_TIMESTAMP
     WHERE rv.id = p_version_id;
    INSERT INTO tracking.route_versions (route_id, effective_from, effective_to, created_by)
    VALUES (v_version.route_id, p_on, v_version.effective_to, v_version.created_by)
    RETURNING tracking.route_versions.id INTO v_target;
    INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
    SELECT v_target, vs.stop_place_id, vs.sequence, vs.planned_offset_min, vs.dwell_sec
    FROM tracking.route_version_stops vs
    WHERE vs.version_id = p_version_id;
    RETURN v_target;
END;
$function$;
