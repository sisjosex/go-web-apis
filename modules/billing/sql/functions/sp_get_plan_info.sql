CREATE OR REPLACE FUNCTION billing.sp_get_plan_info(p_user_id uuid)
 RETURNS TABLE(plan character varying, status character varying, expires_at timestamp with time zone, feature character varying, limit_value integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_plan      VARCHAR;
    v_status    VARCHAR;
    v_expires   TIMESTAMPTZ;
BEGIN
    SELECT
        COALESCE(s.plan, 'free'),
        COALESCE(s.status, 'none'),
        s.expires_at
    INTO v_plan, v_status, v_expires
    FROM billing.subscriptions s
    WHERE s.user_id = p_user_id
      AND s.status IN ('active', 'trial', 'expired')
    ORDER BY
        CASE s.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 ELSE 3 END,
        s.created_at DESC
    LIMIT 1;

    IF v_plan IS NULL THEN
        v_plan   := 'free';
        v_status := 'none';
    END IF;

    RETURN QUERY
    SELECT
        CAST(v_plan   AS VARCHAR),
        CAST(v_status AS VARCHAR),
        v_expires,
        CAST(pl.feature AS VARCHAR),
        pl.limit_value
    FROM billing.plan_limits pl
    WHERE pl.plan = v_plan
    ORDER BY pl.feature;
END;
$function$;
