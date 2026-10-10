CREATE OR REPLACE FUNCTION billing.sp_adjust_tenant_subscription(p_tenant_id uuid, p_plan character varying, p_cycle character varying, p_status character varying, p_expires_at timestamp with time zone, p_reason text, p_adjusted_by uuid)
 RETURNS TABLE(plan character varying, billing_cycle character varying, status character varying, expires_at timestamp with time zone, effective_plan character varying, limits jsonb, prices jsonb, payment_code character varying, pending_payment jsonb)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION billing.sp_adjust_tenant_subscription(uuid,character varying,character varying,character varying,timestamp with time zone,text,uuid) IS 'Sets a business''s plan, cycle, status and end by hand and records the adjustment (before, after, reason, who) in the same transaction; answers the plan as sp_get_tenant_plan (BILLING-002 D3); raises billing.adjust.reason-required, billing.tenant.not-found';
