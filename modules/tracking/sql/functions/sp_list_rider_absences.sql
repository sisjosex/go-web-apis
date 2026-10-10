CREATE OR REPLACE FUNCTION tracking.sp_list_rider_absences(p_tenant_id uuid, p_rider_id uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, rider_id uuid, date_from date, date_to date, direction character varying, reason character varying, reported_by uuid, reported_by_name character varying, reported_via character varying, created_at timestamp without time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    PERFORM tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);
    RETURN QUERY SELECT * FROM tracking.fn_rider_absence_rows(p_tenant_id, p_rider_id, NULL, p_from, p_to);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_absences(uuid,uuid,date,date,uuid,uuid) IS 'A rider''s absences overlapping [p_from, p_to], oldest first, with the reporter''s name; scoped as sp_get_rider_status (TRACK-022)';
