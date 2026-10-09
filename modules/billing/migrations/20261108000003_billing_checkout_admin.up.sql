-- BILLING-002: a friendlier QR checkout, and the platform's view to confirm payments and adjust a plan.
--
-- D1  A QR per price: plan_prices.qr_image_url, the bank QR with the amount already in it. The config's
--     single QR stays the fallback for a price without one.
-- D2  The business's code (its slug, upper case) identifies the payment: the customer pastes it in the
--     transfer's note, "Ya pagué" sends no field, and the notice stores the code as its reference. One
--     pending notice per business: a second one updates the first.
-- D3  A super_admin sets a business's plan, cycle, status and end by hand with a reason; every adjustment
--     writes billing.subscription_adjustments (before, after, reason, who) in the same transaction.
--     A notice can be rejected with a reason.
--
-- Like BILLING-001's, these run on a dedicated tenant's database too: what reads tenancy runs only on the
-- platform's, where the business plans live.

ALTER TABLE billing.plan_prices ADD COLUMN qr_image_url TEXT;

COMMENT ON COLUMN billing.plan_prices.qr_image_url IS
'The bank QR for this exact amount, made once in the bank''s app (BILLING-002 D1); NULL falls back to the configured QR';

ALTER TABLE billing.payments
    ADD COLUMN reject_reason TEXT,
    ADD COLUMN rejected_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    DROP CONSTRAINT IF EXISTS payments_status_check,
    ADD CONSTRAINT payments_status_check
        CHECK (status IN ('pending', 'notified', 'completed', 'failed', 'refunded', 'rejected'));

CREATE TABLE billing.subscription_adjustments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    before      JSONB,
    after       JSONB NOT NULL,
    reason      TEXT NOT NULL,
    adjusted_by UUID REFERENCES auth.users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_subscription_adjustments_tenant ON billing.subscription_adjustments (tenant_id, created_at DESC);

COMMENT ON TABLE billing.subscription_adjustments IS
'Every hand change of a business''s plan by the platform: before and after, the reason and who (BILLING-002 D3)';

-- ===========================================================================
-- The business's plan, with its payment code, the QR per price and the notice under review
-- ===========================================================================

DROP FUNCTION billing.sp_get_tenant_plan(UUID);

CREATE FUNCTION billing.sp_get_tenant_plan(p_tenant_id UUID)
RETURNS TABLE(
    plan VARCHAR, billing_cycle VARCHAR, status VARCHAR, expires_at TIMESTAMPTZ,
    effective_plan VARCHAR, limits JSONB, prices JSONB, payment_code VARCHAR, pending_payment JSONB
) AS $$
DECLARE
    v_sub RECORD;
    v_effective VARCHAR;
    v_code VARCHAR;
BEGIN
    SELECT s.plan, s.billing_cycle, s.status, s.expires_at INTO v_sub
    FROM billing.subscriptions s
    WHERE s.tenant_id = p_tenant_id AND s.status IN ('active', 'trial', 'expired')
    ORDER BY CASE s.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 ELSE 3 END, s.created_at DESC
    LIMIT 1;

    v_effective := CASE WHEN v_sub.status IN ('active', 'trial') THEN v_sub.plan ELSE 'free' END;

    IF to_regclass('tenancy.tenants') IS NOT NULL THEN
        EXECUTE 'SELECT UPPER(t.slug) FROM tenancy.tenants t WHERE t.id = $1' INTO v_code USING p_tenant_id;
    END IF;

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
                'plan', pp.plan, 'cycle', pp.billing_cycle, 'amount', pp.amount, 'currency', pp.currency,
                'qr_image_url', pp.qr_image_url)
                ORDER BY pp.plan, pp.billing_cycle)
            FROM billing.plan_prices pp
        ), '[]'::jsonb),
        v_code,
        (
            SELECT jsonb_build_object(
                'id', p.id, 'plan', p.plan, 'cycle', p.billing_cycle, 'amount', p.amount,
                'currency', p.currency, 'created_at', p.created_at)
            FROM billing.payments p
            WHERE p.tenant_id = p_tenant_id AND p.status = 'notified'
            ORDER BY p.created_at DESC
            LIMIT 1
        );
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION billing.sp_get_tenant_plan(UUID) IS
'A business''s plan, cycle, status and end, the limits it has now (free''s once expired), the price list with each price''s QR, its payment code and the notice under review (BILLING-001, BILLING-002)';

-- ===========================================================================
-- D2: "Ya pagué" with no field; one pending notice per business
-- ===========================================================================

DROP FUNCTION billing.sp_notify_payment(UUID, UUID, VARCHAR, VARCHAR, VARCHAR);

CREATE FUNCTION billing.sp_notify_payment(
    p_tenant_id UUID,
    p_user_id UUID,
    p_plan VARCHAR,
    p_cycle VARCHAR,
    p_reference VARCHAR DEFAULT NULL
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
    v_reference VARCHAR := NULLIF(TRIM(p_reference), '');
BEGIN
    SELECT pp.amount, pp.currency INTO v_price
    FROM billing.plan_prices pp WHERE pp.plan = p_plan AND pp.billing_cycle = p_cycle;
    IF v_price IS NULL THEN
        RAISE EXCEPTION 'billing.price.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_reference IS NULL AND to_regclass('tenancy.tenants') IS NOT NULL THEN
        EXECUTE 'SELECT UPPER(t.slug) FROM tenancy.tenants t WHERE t.id = $1' INTO v_reference USING p_tenant_id;
    END IF;

    -- A second notice while one is under review replaces it: one pending payment per business.
    SELECT p.id INTO v_id FROM billing.payments p
    WHERE p.tenant_id = p_tenant_id AND p.status = 'notified'
    ORDER BY p.created_at DESC
    LIMIT 1
    FOR UPDATE;
    IF v_id IS NOT NULL THEN
        UPDATE billing.payments p SET
            user_id = p_user_id, plan = p_plan, billing_cycle = p_cycle, amount = v_price.amount,
            currency = v_price.currency, reference = v_reference
        WHERE p.id = v_id;
        RETURN QUERY SELECT * FROM billing.fn_payment_row(v_id);
        RETURN;
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
            'notified', 'bank_qr', v_reference)
    RETURNING billing.payments.id INTO v_id;

    RETURN QUERY SELECT * FROM billing.fn_payment_row(v_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_notify_payment(UUID, UUID, VARCHAR, VARCHAR, VARCHAR) IS
'Records a customer''s "Ya pagué" for a plan and cycle, priced from plan_prices, the business''s code as reference when none is sent; a second notice while one is under review updates it (BILLING-001 D3, BILLING-002 D2); raises billing.price.not-found';

-- ===========================================================================
-- D3: reject, the businesses list, the adjustment
-- ===========================================================================

CREATE FUNCTION billing.sp_reject_payment(p_payment_id UUID, p_reason TEXT, p_rejected_by UUID)
RETURNS TABLE(
    id UUID, tenant_id UUID, user_id UUID, plan VARCHAR, billing_cycle VARCHAR, amount NUMERIC,
    currency VARCHAR, status VARCHAR, reference VARCHAR, paid_at TIMESTAMPTZ, period_start TIMESTAMPTZ,
    period_end TIMESTAMPTZ, created_at TIMESTAMPTZ
) AS $$
DECLARE
    v_pay billing.payments%ROWTYPE;
BEGIN
    SELECT * INTO v_pay FROM billing.payments p WHERE p.id = p_payment_id FOR UPDATE;
    IF v_pay.id IS NULL OR v_pay.tenant_id IS NULL THEN
        RAISE EXCEPTION 'billing.payment.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_pay.status <> 'notified' THEN
        RAISE EXCEPTION 'billing.payment.not-pending' USING ERRCODE = 'P0001';
    END IF;

    UPDATE billing.payments p SET status = 'rejected', reject_reason = NULLIF(TRIM(p_reason), ''),
        rejected_by = p_rejected_by
    WHERE p.id = v_pay.id;

    RETURN QUERY SELECT * FROM billing.fn_payment_row(v_pay.id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_reject_payment(UUID, TEXT, UUID) IS
'Rejects a notified payment with a reason; the plan does not change (BILLING-002 D3); raises billing.payment.not-found, billing.payment.not-pending';

-- Who to tell about a payment: the member who notified it and the business's name (D4).
-- plpgsql, not sql: a dedicated tenant's database has no tenancy schema to check the body against.
CREATE FUNCTION billing.sp_payment_contact(p_payment_id UUID)
RETURNS TABLE(email VARCHAR, first_name VARCHAR, tenant_name VARCHAR) AS $$
BEGIN
    RETURN QUERY
    SELECT CAST(u.email AS VARCHAR), CAST(u.first_name AS VARCHAR), CAST(t.name AS VARCHAR)
    FROM billing.payments p
    LEFT JOIN auth.users u ON u.id = p.user_id
    LEFT JOIN tenancy.tenants t ON t.id = p.tenant_id
    WHERE p.id = p_payment_id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION billing.sp_payment_contact(UUID) IS
'The notifier''s email and name and the business''s name, for the payment emails (BILLING-002 D4)';

-- Every business with its current plan and the notice under review, by name, paged (D3).
CREATE FUNCTION billing.sp_list_tenant_subscriptions(
    p_search VARCHAR DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS TABLE(
    tenant_id UUID, tenant_name VARCHAR, tenant_slug VARCHAR, plan VARCHAR, billing_cycle VARCHAR,
    status VARCHAR, expires_at TIMESTAMPTZ, pending_payment_id UUID, total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT t.id, CAST(t.name AS VARCHAR), CAST(t.slug AS VARCHAR),
           CAST(COALESCE(s.plan, 'free') AS VARCHAR), CAST(COALESCE(s.billing_cycle, 'monthly') AS VARCHAR),
           CAST(COALESCE(s.status, 'none') AS VARCHAR), s.expires_at, pp.id,
           COUNT(*) OVER ()
    FROM tenancy.tenants t
    LEFT JOIN LATERAL (
        SELECT x.plan, x.billing_cycle, x.status, x.expires_at FROM billing.subscriptions x
        WHERE x.tenant_id = t.id AND x.status IN ('active', 'trial', 'expired', 'canceled')
        ORDER BY CASE x.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 WHEN 'expired' THEN 3 ELSE 4 END,
                 x.created_at DESC
        LIMIT 1
    ) s ON true
    LEFT JOIN LATERAL (
        SELECT p.id FROM billing.payments p
        WHERE p.tenant_id = t.id AND p.status = 'notified'
        ORDER BY p.created_at DESC
        LIMIT 1
    ) pp ON true
    WHERE p_search IS NULL OR p_search = ''
       OR t.name ILIKE '%' || p_search || '%' OR t.slug ILIKE '%' || p_search || '%'
    ORDER BY t.name, t.id
    LIMIT p_page_size OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION billing.sp_list_tenant_subscriptions(VARCHAR, INT, INT) IS
'Every business with its current plan and its notice under review, by name, paged with total_count (BILLING-002 D3)';

-- The platform sets a business's plan by hand. The row in force is changed in place (a business without
-- one gets it) and the adjustment row says what it was, what it became, why and who — one transaction.
CREATE FUNCTION billing.sp_adjust_tenant_subscription(
    p_tenant_id UUID,
    p_plan VARCHAR,
    p_cycle VARCHAR,
    p_status VARCHAR,
    p_expires_at TIMESTAMPTZ,
    p_reason TEXT,
    p_adjusted_by UUID
)
RETURNS TABLE(
    plan VARCHAR, billing_cycle VARCHAR, status VARCHAR, expires_at TIMESTAMPTZ,
    effective_plan VARCHAR, limits JSONB, prices JSONB, payment_code VARCHAR, pending_payment JSONB
) AS $$
DECLARE
    v_sub billing.subscriptions%ROWTYPE;
    v_before JSONB;
BEGIN
    IF NULLIF(TRIM(p_reason), '') IS NULL THEN
        RAISE EXCEPTION 'billing.adjust.reason-required' USING ERRCODE = 'P0001';
    END IF;
    IF to_regclass('tenancy.tenants') IS NOT NULL THEN
        PERFORM 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'billing.tenant.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    SELECT * INTO v_sub FROM billing.subscriptions s
    WHERE s.tenant_id = p_tenant_id AND s.status IN ('active', 'trial', 'expired', 'canceled')
    ORDER BY CASE s.status WHEN 'active' THEN 1 WHEN 'trial' THEN 2 WHEN 'expired' THEN 3 ELSE 4 END,
             s.created_at DESC
    LIMIT 1
    FOR UPDATE;

    IF v_sub.id IS NULL THEN
        INSERT INTO billing.subscriptions (tenant_id, plan, billing_cycle, status, expires_at)
        VALUES (p_tenant_id, p_plan, p_cycle, p_status, p_expires_at);
    ELSE
        v_before := jsonb_build_object('plan', v_sub.plan, 'cycle', v_sub.billing_cycle, 'status', v_sub.status,
                                       'expires_at', v_sub.expires_at);
        UPDATE billing.subscriptions s SET
            plan = p_plan, billing_cycle = p_cycle, status = p_status, expires_at = p_expires_at,
            canceled_at = CASE WHEN p_status = 'canceled' THEN NOW() END, updated_at = NOW()
        WHERE s.id = v_sub.id;
    END IF;

    INSERT INTO billing.subscription_adjustments (tenant_id, before, after, reason, adjusted_by)
    VALUES (p_tenant_id, v_before,
            jsonb_build_object('plan', p_plan, 'cycle', p_cycle, 'status', p_status, 'expires_at', p_expires_at),
            TRIM(p_reason), p_adjusted_by);

    RETURN QUERY SELECT * FROM billing.sp_get_tenant_plan(p_tenant_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION billing.sp_adjust_tenant_subscription(UUID, VARCHAR, VARCHAR, VARCHAR, TIMESTAMPTZ, TEXT, UUID) IS
'Sets a business''s plan, cycle, status and end by hand and records the adjustment (before, after, reason, who) in the same transaction; answers the plan as sp_get_tenant_plan (BILLING-002 D3); raises billing.adjust.reason-required, billing.tenant.not-found';
