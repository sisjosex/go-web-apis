-- BILLING-001: plans per business, monthly or annual, paid by bank QR and confirmed by the platform.
--
-- D1  A business (tenant) has its own subscription row: plan, cycle, status, expires_at (the paid-up
--     period's end). Its user_id is NULL — who paid is on the payment. A user's own row (tenant_id
--     NULL) keeps only the "tenants" limit, so every user-level SP is untouched.
-- D3  The customer notifies a payment (reference + plan + cycle; amount from plan_prices); a
--     super_admin confirms it, which extends expires_at by the cycle from the later of now and the
--     current end. Seven days after expires_at an active business falls back to the free limits
--     (status expired; nothing is deleted). payments.purpose lets a future POS reuse the same notice.
--
-- Billing migrations also run on a dedicated tenant's database, which has no tenancy schema: tenant_id
-- carries no foreign key, and what reads tenancy runs only where it exists.

-- ===========================================================================
-- Tables
-- ===========================================================================

ALTER TABLE billing.subscriptions
    ADD COLUMN tenant_id UUID,
    ADD COLUMN billing_cycle VARCHAR(10) NOT NULL DEFAULT 'monthly' CHECK (billing_cycle IN ('monthly', 'annual')),
    ALTER COLUMN user_id DROP NOT NULL,
    ADD CONSTRAINT chk_subscription_owner CHECK (user_id IS NOT NULL OR tenant_id IS NOT NULL);

-- One live plan per business, as per user.
CREATE UNIQUE INDEX uq_billing_subscriptions_tenant_active
    ON billing.subscriptions (tenant_id)
    WHERE tenant_id IS NOT NULL AND status IN ('active', 'trial');
-- The expiry job's scan: live paid plans by end date.
CREATE INDEX idx_billing_subscriptions_expiry
    ON billing.subscriptions (expires_at)
    WHERE tenant_id IS NOT NULL AND status = 'active' AND plan <> 'free';

COMMENT ON COLUMN billing.subscriptions.tenant_id IS
'The business the plan is for (BILLING-001 D1); NULL on a user''s own row, which only limits how many businesses they create';

ALTER TABLE billing.payments
    ADD COLUMN tenant_id UUID,
    ADD COLUMN plan VARCHAR(20) CHECK (plan IN ('free', 'pro', 'enterprise')),
    ADD COLUMN billing_cycle VARCHAR(10) CHECK (billing_cycle IN ('monthly', 'annual')),
    ADD COLUMN reference VARCHAR(255),
    ADD COLUMN purpose VARCHAR(30) NOT NULL DEFAULT 'subscription',
    ADD COLUMN confirmed_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    DROP CONSTRAINT IF EXISTS payments_status_check,
    ADD CONSTRAINT payments_status_check CHECK (status IN ('pending', 'notified', 'completed', 'failed', 'refunded'));

-- The platform's queue of payments to confirm, oldest first.
CREATE INDEX idx_billing_payments_notified ON billing.payments (created_at) WHERE status = 'notified';

COMMENT ON COLUMN billing.payments.purpose IS
'What the payment is for: subscription today; a future sale (POS, web shop) reuses the same notice and confirmation (BILLING-001 D3)';

CREATE TABLE billing.plan_prices (
    plan          VARCHAR(20) NOT NULL CHECK (plan IN ('pro', 'enterprise')),
    billing_cycle VARCHAR(10) NOT NULL CHECK (billing_cycle IN ('monthly', 'annual')),
    amount        NUMERIC(10, 2) NOT NULL CHECK (amount > 0),
    currency      VARCHAR(3) NOT NULL DEFAULT 'BOB',
    PRIMARY KEY (plan, billing_cycle)
);

COMMENT ON TABLE billing.plan_prices IS
'What a plan costs per cycle; annual = 10 months (two free). Enterprise has no row: it is sold by contact (BILLING-001 D4)';

INSERT INTO billing.plan_prices (plan, billing_cycle, amount, currency) VALUES
    ('pro', 'monthly', 200, 'BOB'),
    ('pro', 'annual', 2000, 'BOB')
ON CONFLICT (plan, billing_cycle) DO UPDATE SET amount = EXCLUDED.amount, currency = EXCLUDED.currency;

-- The limits each business feature has (D2). tracking_items is kept for older readers.
INSERT INTO billing.plan_limits (plan, feature, limit_value) VALUES
    ('free', 'tracking_riders', 50),
    ('free', 'tracking_vehicles', 3),
    ('pro', 'tracking_riders', 1000),
    ('pro', 'tracking_vehicles', 50),
    ('enterprise', 'tracking_riders', -1),
    ('enterprise', 'tracking_vehicles', -1)
ON CONFLICT (plan, feature) DO UPDATE SET limit_value = EXCLUDED.limit_value;

-- ===========================================================================
-- Backfill: each business starts on its owner's current plan
-- ===========================================================================

DO $$
BEGIN
    IF to_regclass('tenancy.tenants') IS NOT NULL THEN
        INSERT INTO billing.subscriptions (tenant_id, user_id, plan, status, started_at, expires_at)
        SELECT t.id, NULL, COALESCE(s.plan, 'free'), 'active', COALESCE(s.started_at, NOW()), s.expires_at
        FROM tenancy.tenants t
        LEFT JOIN LATERAL (
            SELECT tu.user_id FROM tenancy.tenant_users tu
            WHERE tu.tenant_id = t.id AND tu.role = 'owner'
            ORDER BY tu.joined_at
            LIMIT 1
        ) o ON true
        LEFT JOIN LATERAL (
            SELECT s.plan, s.started_at, s.expires_at FROM billing.subscriptions s
            WHERE s.user_id = o.user_id AND s.tenant_id IS NULL AND s.status IN ('active', 'trial')
            ORDER BY s.created_at DESC
            LIMIT 1
        ) s ON true
        WHERE NOT EXISTS (
            SELECT 1 FROM billing.subscriptions x WHERE x.tenant_id = t.id AND x.status IN ('active', 'trial')
        );
    END IF;
END;
$$;

-- ===========================================================================
-- A business's plan
-- ===========================================================================

-- The plan a business is on and the limits it has now: an expired plan has the free plan's limits
-- (D3 grace is the job's: status stays active for 7 days past expires_at). A business without a row
-- is on free.
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

-- ===========================================================================
-- D3: notify, confirm, expire
-- ===========================================================================

CREATE FUNCTION billing.fn_payment_row(p_id UUID)
RETURNS TABLE(
    id UUID, tenant_id UUID, user_id UUID, plan VARCHAR, billing_cycle VARCHAR, amount NUMERIC,
    currency VARCHAR, status VARCHAR, reference VARCHAR, paid_at TIMESTAMPTZ, period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ, created_at TIMESTAMPTZ
) AS $$
    SELECT p.id, p.tenant_id, p.user_id, CAST(p.plan AS VARCHAR), CAST(p.billing_cycle AS VARCHAR), p.amount,
           CAST(p.currency AS VARCHAR), CAST(p.status AS VARCHAR), CAST(p.reference AS VARCHAR), p.paid_at,
           p.period_start, p.period_end, p.created_at
    FROM billing.payments p WHERE p.id = p_id;
$$ LANGUAGE sql STABLE;

-- "Ya pagué": records what the customer says they paid, priced from plan_prices, for the platform to
-- confirm. The business's subscription row is created (free) when it has none, since a payment hangs
-- off one.
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

-- The platform's confirmation: the payment is completed and the business's plan runs one more cycle
-- from the later of now and its current end, on the plan paid for.
CREATE FUNCTION billing.sp_confirm_payment(p_payment_id UUID, p_confirmed_by UUID)
RETURNS TABLE(
    id UUID, tenant_id UUID, user_id UUID, plan VARCHAR, billing_cycle VARCHAR, amount NUMERIC,
    currency VARCHAR, status VARCHAR, reference VARCHAR, paid_at TIMESTAMPTZ, period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ, created_at TIMESTAMPTZ
) AS $$
DECLARE
    v_pay billing.payments%ROWTYPE;
    v_sub billing.subscriptions%ROWTYPE;
    v_from TIMESTAMPTZ;
    v_to TIMESTAMPTZ;
BEGIN
    SELECT * INTO v_pay FROM billing.payments p WHERE p.id = p_payment_id FOR UPDATE;
    IF v_pay.id IS NULL OR v_pay.tenant_id IS NULL THEN
        RAISE EXCEPTION 'billing.payment.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_pay.status <> 'notified' THEN
        RAISE EXCEPTION 'billing.payment.not-pending' USING ERRCODE = 'P0001';
    END IF;

    SELECT * INTO v_sub FROM billing.subscriptions s WHERE s.id = v_pay.subscription_id FOR UPDATE;
    v_from := GREATEST(NOW(), COALESCE(CASE WHEN v_sub.status = 'active' AND v_sub.plan = v_pay.plan
        THEN v_sub.expires_at END, NOW()));
    v_to := v_from + CASE v_pay.billing_cycle WHEN 'annual' THEN INTERVAL '1 year' ELSE INTERVAL '1 month' END;

    UPDATE billing.subscriptions s SET
        plan = v_pay.plan, billing_cycle = v_pay.billing_cycle, status = 'active',
        expires_at = v_to, updated_at = NOW()
    WHERE s.id = v_sub.id;

    UPDATE billing.payments p SET
        status = 'completed', paid_at = NOW(), period_start = v_from, period_end = v_to,
        confirmed_by = p_confirmed_by
    WHERE p.id = v_pay.id;

    RETURN QUERY SELECT * FROM billing.fn_payment_row(v_pay.id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_confirm_payment(UUID, UUID) IS
'Confirms a notified payment: completed, and the business''s plan extended one cycle from the later of now and its end (BILLING-001 D3); raises billing.payment.not-found, billing.payment.not-pending';

-- The platform's queue: notified payments, oldest first, with the business's name.
CREATE FUNCTION billing.sp_list_notified_payments()
RETURNS TABLE(
    id UUID, tenant_id UUID, tenant_name VARCHAR, user_id UUID, plan VARCHAR, billing_cycle VARCHAR,
    amount NUMERIC, currency VARCHAR, reference VARCHAR, created_at TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT p.id, p.tenant_id, CAST(t.name AS VARCHAR), p.user_id, CAST(p.plan AS VARCHAR),
           CAST(p.billing_cycle AS VARCHAR), p.amount, CAST(p.currency AS VARCHAR), CAST(p.reference AS VARCHAR),
           p.created_at
    FROM billing.payments p
    INNER JOIN tenancy.tenants t ON t.id = p.tenant_id
    WHERE p.status = 'notified'
    ORDER BY p.created_at;
END;
$$ LANGUAGE plpgsql STABLE;

-- The daily job: a paid plan seven days past its end expires; its business keeps every row and gets
-- the free limits until a payment is confirmed.
CREATE FUNCTION billing.sp_expire_subscriptions(p_grace_days INT DEFAULT 7)
RETURNS INT AS $$
DECLARE
    v_count INT;
BEGIN
    UPDATE billing.subscriptions s SET status = 'expired', updated_at = NOW()
    WHERE s.tenant_id IS NOT NULL AND s.status = 'active' AND s.plan <> 'free'
      AND s.expires_at < NOW() - make_interval(days => p_grace_days);
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_expire_subscriptions(INT) IS
'Expires the paid business plans past their end plus p_grace_days (BILLING-001 D3); answers how many';
