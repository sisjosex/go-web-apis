CREATE OR REPLACE FUNCTION billing.sp_confirm_payment(p_payment_id uuid, p_confirmed_by uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, user_id uuid, plan character varying, billing_cycle character varying, amount numeric, currency character varying, status character varying, reference character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION billing.sp_confirm_payment(uuid,uuid) IS 'Confirms a notified payment: completed, and the business''s plan extended one cycle from the later of now and its end (BILLING-001 D3); raises billing.payment.not-found, billing.payment.not-pending';
