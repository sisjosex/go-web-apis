CREATE OR REPLACE FUNCTION billing.private_check_feature_limit(p_user_id uuid, p_feature character varying, p_count integer)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_plan  VARCHAR;
    v_limit INTEGER;
BEGIN
    -- Resolve active plan (default: free)
    SELECT COALESCE(s.plan, 'free')
    INTO v_plan
    FROM billing.subscriptions s
    WHERE s.user_id = p_user_id
      AND s.status IN ('active', 'trial')
    ORDER BY s.created_at DESC
    LIMIT 1;

    IF v_plan IS NULL THEN
        v_plan := 'free';
    END IF;

    -- Look up limit for this plan + feature
    SELECT pl.limit_value
    INTO v_limit
    FROM billing.plan_limits pl
    WHERE pl.plan    = v_plan
      AND pl.feature = p_feature;

    -- Feature not in plan_limits → no restriction
    IF v_limit IS NULL THEN
        RETURN TRUE;
    END IF;

    -- -1 = unlimited
    IF v_limit = -1 THEN
        RETURN TRUE;
    END IF;

    RETURN p_count < v_limit;
END;
$function$;
