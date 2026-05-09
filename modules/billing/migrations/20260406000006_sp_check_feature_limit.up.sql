-- Migration: sp_check_feature_limit
-- Module: billing
-- Private helper called by other modules' SPs to enforce plan-based limits.
-- Returns TRUE if the action is allowed, FALSE if the limit is reached.
-- Created: 2026-04-06

CREATE OR REPLACE FUNCTION billing.private_check_feature_limit(
    p_user_id UUID,
    p_feature VARCHAR,
    p_count   INTEGER  -- current usage count (before the new item is added)
) RETURNS BOOLEAN AS $$
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
$$ LANGUAGE plpgsql;
