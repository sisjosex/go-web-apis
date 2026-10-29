CREATE OR REPLACE FUNCTION tracking.sp_create_rider_absences_batch(p_tenant_id uuid, p_rider_id uuid, p_ranges jsonb, p_direction character varying DEFAULT NULL::character varying, p_reason character varying DEFAULT NULL::character varying, p_user_id uuid DEFAULT NULL::uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, rider_id uuid, date_from date, date_to date, direction character varying, reason character varying, reported_by uuid, reported_by_name character varying, reported_via character varying, created_at timestamp without time zone, cancelled_tasks integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_range JSONB;
    v_from  DATE;
    v_to    DATE;
BEGIN
    IF jsonb_typeof(p_ranges) IS DISTINCT FROM 'array' OR jsonb_array_length(p_ranges) NOT BETWEEN 1 AND 31 THEN
        RAISE EXCEPTION 'absence.invalid-range' USING ERRCODE = 'P0001';
    END IF;

    -- Each range through the one absence rule there is; a refusal on any of them raises out of this call,
    -- so the ranges already created roll back with it: all or nothing (MOBILE-038 D2).
    FOR v_range IN SELECT r.value FROM jsonb_array_elements(p_ranges) AS r LOOP
        v_from := CAST(v_range->>'date_from' AS DATE);
        v_to := CAST(v_range->>'date_to' AS DATE);
        BEGIN
            RETURN QUERY
            SELECT * FROM tracking.sp_create_rider_absence(
                p_tenant_id, p_rider_id, v_from, v_to, p_direction, p_reason, p_user_id, p_scope_user_id, p_guardian_user_id);
        EXCEPTION WHEN raise_exception THEN
            -- The refusal keeps its code; the detail names the first day it hit, for the phone to say.
            RAISE EXCEPTION USING ERRCODE = 'P0001', MESSAGE = SQLERRM, DETAIL = to_char(v_from, 'YYYY-MM-DD');
        END;
    END LOOP;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_rider_absences_batch(uuid,uuid,jsonb,character varying,character varying,uuid,uuid,uuid) IS 'Records 1–31 absence ranges of one rider in one call through sp_create_rider_absence, all or nothing: a cut-off, overlap or scope refusal on any range rolls every range back and raises with DETAIL = that range''s first day (MOBILE-038 D2)';
