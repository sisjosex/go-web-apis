-- BILLING-002 down: the BILLING-001 notice and plan read, no adjustments, no QR per price, no rejects.
DROP FUNCTION IF EXISTS billing.sp_adjust_tenant_subscription(UUID, VARCHAR, VARCHAR, VARCHAR, TIMESTAMPTZ, TEXT, UUID);
DROP FUNCTION IF EXISTS billing.sp_list_tenant_subscriptions(VARCHAR, INT, INT);
DROP FUNCTION IF EXISTS billing.sp_payment_contact(UUID);
DROP FUNCTION IF EXISTS billing.sp_reject_payment(UUID, TEXT, UUID);
DROP TABLE IF EXISTS billing.subscription_adjustments;
DROP FUNCTION billing.sp_get_tenant_plan(UUID);
DROP FUNCTION billing.sp_notify_payment(UUID, UUID, VARCHAR, VARCHAR, VARCHAR);

CREATE FUNCTION billing.sp_get_tenant_plan(p_tenant_id UUID)
RETURNS TABLE(
    plan VARCHAR, billing_cycle VARCHAR, status VARCHAR, expires_at TIMESTAMPTZ,
    effective_plan VARCHAR, limits JSONB, prices JSONB
) AS $$
DECLARE
    v_sub RECORD;
    v_effective VARCHAR;
BEGIN
    SELECT s.plan, s.billing_cycle, s.status, s.expires_at INTO v_sub
    FROM billing.subscriptions s
    WHERE s.tenant_id = p_tenant_id AND s.status IN ('active', 'trial', 'expired')
    ORDER BY CASE s.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 ELSE 3 END, s.created_at DESC
    LIMIT 1;

    v_effective := CASE WHEN v_sub.status IN ('active', 'trial') THEN v_sub.plan ELSE 'free' END;

    RETURN QUERY SELECT
        CAST(COALESCE(v_sub.plan, 'free') AS VARCHAR),
        CAST(COALESCE(v_sub.billing_cycle, 'monthly') AS VARCHAR),
        CAST(COALESCE(v_sub.status, 'none') AS VARCHAR),
        v_sub.expires_at,
        CAST(COALESCE(v_effective, 'free') AS VARCHAR),
        COALESCE((
            SELECT jsonb_object_agg(pl.feature, pl.limit_value) FROM billing.plan_limits pl
            WHERE pl.plan = COALESCE(v_effective, 'free')
        ), '{}'::jsonb),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'plan', pp.plan, 'cycle', pp.billing_cycle, 'amount', pp.amount, 'currency', pp.currency)
                ORDER BY pp.plan, pp.billing_cycle)
            FROM billing.plan_prices pp
        ), '[]'::jsonb);
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION billing.sp_get_tenant_plan(UUID) IS
'A business''s plan, cycle, status and end, the limits it has now (free''s once expired) and the price list (BILLING-001)';

CREATE FUNCTION billing.sp_notify_payment(
    p_tenant_id UUID,
    p_user_id UUID,
    p_plan VARCHAR,
    p_cycle VARCHAR,
    p_reference VARCHAR
)
RETURNS TABLE(
    id UUID, tenant_id UUID, user_id UUID, plan VARCHAR, billing_cycle VARCHAR, amount NUMERIC,
    currency VARCHAR, status VARCHAR, reference VARCHAR, paid_at TIMESTAMPTZ, period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ, created_at TIMESTAMPTZ
) AS $$
DECLARE
    v_price RECORD;
    v_sub UUID;
    v_id UUID;
BEGIN
    SELECT pp.amount, pp.currency INTO v_price
    FROM billing.plan_prices pp WHERE pp.plan = p_plan AND pp.billing_cycle = p_cycle;
    IF v_price IS NULL THEN
        RAISE EXCEPTION 'billing.price.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT s.id INTO v_sub FROM billing.subscriptions s
    WHERE s.tenant_id = p_tenant_id AND s.status IN ('active', 'trial', 'expired')
    ORDER BY CASE s.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 ELSE 3 END, s.created_at DESC
    LIMIT 1;
    IF v_sub IS NULL THEN
        INSERT INTO billing.subscriptions (tenant_id, plan, status) VALUES (p_tenant_id, 'free', 'active')
        RETURNING billing.subscriptions.id INTO v_sub;
    END IF;

    INSERT INTO billing.payments (subscription_id, user_id, tenant_id, plan, billing_cycle, amount, currency,
                                  status, provider, reference)
    VALUES (v_sub, p_user_id, p_tenant_id, p_plan, p_cycle, v_price.amount, v_price.currency,
            'notified', 'bank_qr', NULLIF(TRIM(p_reference), ''))
    RETURNING billing.payments.id INTO v_id;

    RETURN QUERY SELECT * FROM billing.fn_payment_row(v_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_notify_payment(UUID, UUID, VARCHAR, VARCHAR, VARCHAR) IS
'Records a customer''s "Ya pagué" for a plan and cycle, priced from plan_prices, waiting for the platform''s confirmation (BILLING-001 D3); raises billing.price.not-found';

UPDATE billing.payments SET status = 'failed' WHERE status = 'rejected';
ALTER TABLE billing.payments
    DROP CONSTRAINT IF EXISTS payments_status_check,
    ADD CONSTRAINT payments_status_check CHECK (status IN ('pending', 'notified', 'completed', 'failed', 'refunded')),
    DROP COLUMN IF EXISTS rejected_by,
    DROP COLUMN IF EXISTS reject_reason;
ALTER TABLE billing.plan_prices DROP COLUMN IF EXISTS qr_image_url;
