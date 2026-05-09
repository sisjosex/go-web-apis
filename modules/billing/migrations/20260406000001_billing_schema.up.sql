-- Migration: billing_schema
-- Module: billing
-- Created: 2026-04-06

-- Create billing schema
CREATE SCHEMA IF NOT EXISTS billing;

-- Subscriptions table: one active sub per user enforced via partial unique index
CREATE TABLE IF NOT EXISTS billing.subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    plan                VARCHAR(20) NOT NULL DEFAULT 'free'
                            CHECK (plan IN ('free', 'pro', 'enterprise')),
    status              VARCHAR(20) NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'expired', 'canceled', 'trial')),
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ,
    canceled_at         TIMESTAMPTZ,
    stripe_sub_id       VARCHAR(255),
    stripe_customer_id  VARCHAR(255),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only one active/trial subscription per user
CREATE UNIQUE INDEX IF NOT EXISTS uq_billing_subscriptions_user_active
    ON billing.subscriptions (user_id)
    WHERE status IN ('active', 'trial');

CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_user_id ON billing.subscriptions (user_id);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_status  ON billing.subscriptions (status);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_plan    ON billing.subscriptions (plan);

-- Payments table: full payment history
CREATE TABLE IF NOT EXISTS billing.payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id     UUID NOT NULL REFERENCES billing.subscriptions(id) ON DELETE CASCADE,
    user_id             UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    amount              NUMERIC(10, 2) NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'USD',
    status              VARCHAR(20) NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'completed', 'failed', 'refunded')),
    provider            VARCHAR(50),
    provider_payment_id VARCHAR(255),
    paid_at             TIMESTAMPTZ,
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_payments_subscription_id ON billing.payments (subscription_id);
CREATE INDEX IF NOT EXISTS idx_billing_payments_user_id         ON billing.payments (user_id);
CREATE INDEX IF NOT EXISTS idx_billing_payments_status          ON billing.payments (status);

-- Plan limits: seeded data — SPs and Go services check this instead of hardcoded constants
CREATE TABLE IF NOT EXISTS billing.plan_limits (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan        VARCHAR(20)  NOT NULL CHECK (plan IN ('free', 'pro', 'enterprise')),
    feature     VARCHAR(100) NOT NULL,
    limit_value INTEGER      NOT NULL DEFAULT -1, -- -1 = unlimited
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (plan, feature)
);

-- Seed plan limits
INSERT INTO billing.plan_limits (plan, feature, limit_value) VALUES
    -- Free plan
    ('free', 'tenants',           1),
    ('free', 'users_per_tenant',  5),
    ('free', 'inventory_items',   100),
    ('free', 'sales_orders',      50),
    ('free', 'tracking_items',    50),
    -- Pro plan
    ('pro',  'tenants',           5),
    ('pro',  'users_per_tenant',  50),
    ('pro',  'inventory_items',   5000),
    ('pro',  'sales_orders',      -1),
    ('pro',  'tracking_items',    -1),
    -- Enterprise plan (-1 = unlimited)
    ('enterprise', 'tenants',           -1),
    ('enterprise', 'users_per_tenant',  -1),
    ('enterprise', 'inventory_items',   -1),
    ('enterprise', 'sales_orders',      -1),
    ('enterprise', 'tracking_items',    -1)
ON CONFLICT (plan, feature) DO UPDATE SET limit_value = EXCLUDED.limit_value;

COMMENT ON TABLE billing.subscriptions IS 'User subscription records — one active per user';
COMMENT ON TABLE billing.payments      IS 'Payment history for subscriptions';
COMMENT ON TABLE billing.plan_limits   IS 'Plan feature limit definitions — seeded, checked by SPs and Go services';
