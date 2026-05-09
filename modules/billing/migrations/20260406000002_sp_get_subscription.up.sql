-- Migration: sp_get_subscription
-- Module: billing
-- Created: 2026-04-06

CREATE OR REPLACE FUNCTION billing.sp_get_subscription(
    p_user_id UUID
) RETURNS TABLE (
    id                  UUID,
    user_id             UUID,
    plan                VARCHAR,
    status              VARCHAR,
    started_at          TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ,
    canceled_at         TIMESTAMPTZ,
    stripe_sub_id       VARCHAR,
    stripe_customer_id  VARCHAR,
    created_at          TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        CAST(s.plan AS VARCHAR),
        CAST(s.status AS VARCHAR),
        s.started_at,
        s.expires_at,
        s.canceled_at,
        CAST(s.stripe_sub_id AS VARCHAR),
        CAST(s.stripe_customer_id AS VARCHAR),
        s.created_at,
        s.updated_at
    FROM billing.subscriptions s
    WHERE s.user_id = p_user_id
    ORDER BY
        CASE WHEN s.status IN ('active', 'trial') THEN 0 ELSE 1 END,
        s.created_at DESC
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;
