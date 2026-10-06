-- BILLING-001 down: business plans, prices and payment notices go; user plans stay as they were.

DROP FUNCTION IF EXISTS billing.sp_expire_subscriptions(INT);
DROP FUNCTION IF EXISTS billing.sp_count_tenant_members(UUID);
DROP FUNCTION IF EXISTS billing.sp_list_notified_payments();
DROP FUNCTION IF EXISTS billing.sp_confirm_payment(UUID, UUID);
DROP FUNCTION IF EXISTS billing.sp_notify_payment(UUID, UUID, VARCHAR, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS billing.fn_payment_row(UUID);
DROP FUNCTION IF EXISTS billing.sp_get_tenant_plan(UUID);

DELETE FROM billing.plan_limits WHERE feature IN ('tracking_riders', 'tracking_vehicles');
DROP TABLE IF EXISTS billing.plan_prices;

DELETE FROM billing.payments WHERE tenant_id IS NOT NULL;
DROP INDEX IF EXISTS billing.idx_billing_payments_notified;
ALTER TABLE billing.payments
    DROP CONSTRAINT IF EXISTS payments_status_check,
    ADD CONSTRAINT payments_status_check CHECK (status IN ('pending', 'completed', 'failed', 'refunded')),
    DROP COLUMN confirmed_by,
    DROP COLUMN purpose,
    DROP COLUMN reference,
    DROP COLUMN billing_cycle,
    DROP COLUMN plan,
    DROP COLUMN tenant_id;

DELETE FROM billing.subscriptions WHERE tenant_id IS NOT NULL;
DROP INDEX IF EXISTS billing.idx_billing_subscriptions_expiry;
DROP INDEX IF EXISTS billing.uq_billing_subscriptions_tenant_active;
ALTER TABLE billing.subscriptions
    DROP CONSTRAINT IF EXISTS chk_subscription_owner,
    ALTER COLUMN user_id SET NOT NULL,
    DROP COLUMN billing_cycle,
    DROP COLUMN tenant_id;
