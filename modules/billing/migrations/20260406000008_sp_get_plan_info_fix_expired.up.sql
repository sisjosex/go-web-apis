-- Migration: sp_get_plan_info (fix expired status)
-- Module: billing
-- Fix: also consider 'expired' subscriptions so is_expired resolves correctly.
-- Priority: active > trial > expired. Falls back to free/none if none found.

CREATE OR REPLACE FUNCTION billing.sp_get_plan_info(
    p_user_id UUID
) RETURNS TABLE (
    plan        VARCHAR,
    status      VARCHAR,
    expires_at  TIMESTAMPTZ,
    feature     VARCHAR,
    limit_value INTEGER
) AS $$
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
$$ LANGUAGE plpgsql;
