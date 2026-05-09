-- Migration: sp_upsert_subscription
-- Module: billing
-- Created: 2026-04-06

CREATE OR REPLACE FUNCTION billing.sp_upsert_subscription(
    p_user_id             UUID,
    p_plan                VARCHAR,
    p_status              VARCHAR     DEFAULT 'active',
    p_expires_at          TIMESTAMPTZ DEFAULT NULL,
    p_stripe_sub_id       VARCHAR     DEFAULT NULL,
    p_stripe_customer_id  VARCHAR     DEFAULT NULL
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
#variable_conflict use_column
DECLARE
    v_sub_id UUID;
BEGIN
    -- Validate plan
    IF p_plan NOT IN ('free', 'pro', 'enterprise') THEN
        RAISE EXCEPTION 'billing.subscription.invalid-plan'
            USING ERRCODE = 'P0001';
    END IF;

    -- Validate status
    IF p_status NOT IN ('active', 'expired', 'canceled', 'trial') THEN
        RAISE EXCEPTION 'billing.subscription.invalid-status'
            USING ERRCODE = 'P0001';
    END IF;

    -- Cancel existing active/trial subscription if switching to a different plan
    UPDATE billing.subscriptions s
    SET
        status      = 'canceled',
        canceled_at = NOW(),
        updated_at  = NOW()
    WHERE s.user_id = p_user_id
      AND s.status  IN ('active', 'trial')
      AND s.plan   != p_plan;

    -- Insert or update the subscription for this plan
    INSERT INTO billing.subscriptions (
        user_id, plan, status, expires_at, stripe_sub_id, stripe_customer_id
    )
    VALUES (
        p_user_id, p_plan, p_status, p_expires_at, p_stripe_sub_id, p_stripe_customer_id
    )
    ON CONFLICT (user_id) WHERE status IN ('active', 'trial')
    DO UPDATE SET
        plan               = EXCLUDED.plan,
        status             = EXCLUDED.status,
        expires_at         = EXCLUDED.expires_at,
        stripe_sub_id      = COALESCE(EXCLUDED.stripe_sub_id,      billing.subscriptions.stripe_sub_id),
        stripe_customer_id = COALESCE(EXCLUDED.stripe_customer_id, billing.subscriptions.stripe_customer_id),
        updated_at         = NOW()
    RETURNING billing.subscriptions.id INTO v_sub_id;

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
    WHERE s.id = v_sub_id;
END;
$$ LANGUAGE plpgsql;

