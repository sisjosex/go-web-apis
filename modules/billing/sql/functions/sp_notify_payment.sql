CREATE OR REPLACE FUNCTION billing.sp_notify_payment(p_tenant_id uuid, p_user_id uuid, p_plan character varying, p_cycle character varying, p_reference character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, tenant_id uuid, user_id uuid, plan character varying, billing_cycle character varying, amount numeric, currency character varying, status character varying, reference character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION billing.sp_notify_payment(uuid,uuid,character varying,character varying,character varying) IS 'Records a customer''s "Ya pagué" for a plan and cycle, priced from plan_prices, the business''s code as reference when none is sent; a second notice while one is under review updates it (BILLING-001 D3, BILLING-002 D2); raises billing.price.not-found';
