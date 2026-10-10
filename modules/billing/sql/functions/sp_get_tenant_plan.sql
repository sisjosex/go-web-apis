CREATE OR REPLACE FUNCTION billing.sp_get_tenant_plan(p_tenant_id uuid)
 RETURNS TABLE(plan character varying, billing_cycle character varying, status character varying, expires_at timestamp with time zone, effective_plan character varying, limits jsonb, prices jsonb, payment_code character varying, pending_payment jsonb)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION billing.sp_get_tenant_plan(uuid) IS 'A business''s plan, cycle, status and end, the limits it has now (free''s once expired), the price list with each price''s QR, its payment code and the notice under review (BILLING-001, BILLING-002)';
