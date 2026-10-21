CREATE OR REPLACE FUNCTION tracking.sp_delete_rider_absence(p_tenant_id uuid, p_rider_id uuid, p_absence_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_cutoff INT;
    v_absence tracking.rider_absences%ROWTYPE;
BEGIN
    v_cutoff := tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id, true);

    SELECT a.* INTO v_absence
    FROM tracking.rider_absences a
    WHERE a.id = p_absence_id AND a.rider_id = p_rider_id AND a.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_absence.id IS NULL THEN
        RAISE EXCEPTION 'absence.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_guardian_user_id IS NOT NULL
       AND tracking.fn_absence_first_pickup(p_tenant_id, p_rider_id, v_absence.date_from, v_absence.date_to, v_absence.direction)
           <= now() + make_interval(mins => v_cutoff) THEN
        RAISE EXCEPTION 'absence.cutoff' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_absences a WHERE a.id = p_absence_id;
    PERFORM tracking.fn_absence_routes_changed(p_rider_id, v_absence.date_from, v_absence.date_to, v_absence.direction);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_rider_absence(uuid,uuid,uuid,uuid,uuid) IS 'Deletes a rider''s absence and writes route.changed so planned trips regain the rider; raises absence.not-found, and absence.cutoff for a guardian inside the organization''s cut-off (TRACK-022)';
