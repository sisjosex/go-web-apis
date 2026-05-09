-- Migration: sp_record_payment
-- Module: billing
-- Created: 2026-04-06

CREATE OR REPLACE FUNCTION billing.sp_record_payment(
    p_subscription_id     UUID,
    p_user_id             UUID,
    p_amount              NUMERIC,
    p_currency            VARCHAR     DEFAULT 'USD',
    p_status              VARCHAR     DEFAULT 'completed',
    p_provider            VARCHAR     DEFAULT NULL,
    p_provider_payment_id VARCHAR     DEFAULT NULL,
    p_period_start        TIMESTAMPTZ DEFAULT NULL,
    p_period_end          TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE (
    id                  UUID,
    subscription_id     UUID,
    user_id             UUID,
    amount              NUMERIC,
    currency            VARCHAR,
    status              VARCHAR,
    provider            VARCHAR,
    provider_payment_id VARCHAR,
    paid_at             TIMESTAMPTZ,
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ
) AS $$
BEGIN
    -- Validate subscription belongs to user
    IF NOT EXISTS (
        SELECT 1 FROM billing.subscriptions s
        WHERE s.id = p_subscription_id AND s.user_id = p_user_id
    ) THEN
        RAISE EXCEPTION 'billing.payment.subscription-not-found'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO billing.payments (
        subscription_id, user_id, amount, currency, status,
        provider, provider_payment_id,
        paid_at, period_start, period_end
    )
    VALUES (
        p_subscription_id, p_user_id, p_amount, p_currency, p_status,
        p_provider, p_provider_payment_id,
        CASE WHEN p_status = 'completed' THEN NOW() ELSE NULL END,
        p_period_start, p_period_end
    )
    RETURNING
        billing.payments.id,
        billing.payments.subscription_id,
        billing.payments.user_id,
        billing.payments.amount,
        CAST(billing.payments.currency            AS VARCHAR),
        CAST(billing.payments.status              AS VARCHAR),
        CAST(billing.payments.provider            AS VARCHAR),
        CAST(billing.payments.provider_payment_id AS VARCHAR),
        billing.payments.paid_at,
        billing.payments.period_start,
        billing.payments.period_end,
        billing.payments.created_at;
END;
$$ LANGUAGE plpgsql;
