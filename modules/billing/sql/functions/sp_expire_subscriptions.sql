CREATE OR REPLACE FUNCTION billing.sp_expire_subscriptions(p_grace_days integer DEFAULT 7)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    UPDATE billing.subscriptions s SET status = 'expired', updated_at = NOW()
    WHERE s.tenant_id IS NOT NULL AND s.status = 'active' AND s.plan <> 'free'
      AND s.expires_at < NOW() - make_interval(days => p_grace_days);
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION billing.sp_expire_subscriptions(integer) IS 'Expires the paid business plans past their end plus p_grace_days (BILLING-001 D3); answers how many';
