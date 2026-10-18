CREATE OR REPLACE FUNCTION tracking.sp_rider_absence_cutoffs(p_tenant_id uuid, p_rider_id uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_cutoff INT;
    v_from   DATE := p_from;
    v_to     DATE := p_to;
BEGIN
    v_cutoff := tracking.fn_rider_absence_guard(p_tenant_id, p_rider_id, p_scope_user_id, p_guardian_user_id);

    -- Defaults: two weeks from the rider's local today.
    IF v_from IS NULL THEN
        SELECT CAST(now() AT TIME ZONE o.timezone AS DATE) INTO v_from
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id;
    END IF;
    v_to := COALESCE(v_to, v_from + 13);
    IF v_to < v_from OR v_to - v_from > 30 THEN
        RAISE EXCEPTION 'rider.cutoffs-range' USING ERRCODE = 'P0001';
    END IF;

    -- The rule sp_create_rider_absence raises absence.cutoff on, day by day, so the two never disagree.
    RETURN jsonb_build_object(
        'cutoff_min', v_cutoff,
        'days', (
            SELECT jsonb_agg(jsonb_build_object(
                'date',      d.day,
                'has_trip',  p.first_pickup IS NOT NULL,
                'cutoff_at', p.first_pickup - make_interval(mins => v_cutoff),
                'closed',    COALESCE(p.first_pickup - make_interval(mins => v_cutoff) <= now(), FALSE)
            ) ORDER BY d.day)
            FROM generate_series(v_from, v_to, INTERVAL '1 day') g
            CROSS JOIN LATERAL (SELECT CAST(g AS DATE) AS day) d
            CROSS JOIN LATERAL (
                SELECT tracking.fn_absence_first_pickup(p_tenant_id, p_rider_id, d.day, d.day, NULL) AS first_pickup
            ) p
        )
    );
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_absence_cutoffs(uuid,uuid,date,date,uuid,uuid) IS 'Per day of [p_from, p_to] (default today and the next 13, at most 31): whether the rider has a materialised pickup, the guardian''s absence cut-off before it and whether it has passed — fn_absence_first_pickup less absence_cutoff_min, the rule of absence.cutoff; raises rider.cutoffs-range; scoped as sp_get_rider_status (MOBILE-009 D2, TRACK-061)';
