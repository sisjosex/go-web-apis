-- Migration: sp_list_payments
-- Module: billing
-- Created: 2026-04-06

CREATE OR REPLACE FUNCTION billing.sp_list_payments(
    p_user_id UUID,
    p_page    INTEGER DEFAULT 1,
    p_limit   INTEGER DEFAULT 10
) RETURNS TABLE (
    id                  UUID,
    subscription_id     UUID,
    amount              NUMERIC,
    currency            VARCHAR,
    status              VARCHAR,
    provider            VARCHAR,
    provider_payment_id VARCHAR,
    paid_at             TIMESTAMPTZ,
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ,
    total_count         BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        p.subscription_id,
        p.amount,
        CAST(p.currency            AS VARCHAR),
        CAST(p.status              AS VARCHAR),
        CAST(p.provider            AS VARCHAR),
        CAST(p.provider_payment_id AS VARCHAR),
        p.paid_at,
        p.period_start,
        p.period_end,
        p.created_at,
        COUNT(*) OVER() AS total_count
    FROM billing.payments p
    WHERE p.user_id = p_user_id
    ORDER BY p.created_at DESC
    OFFSET (p_page - 1) * p_limit
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;
