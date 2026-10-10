CREATE OR REPLACE FUNCTION billing.sp_list_tenant_subscriptions(p_search character varying DEFAULT NULL::character varying, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(tenant_id uuid, tenant_name character varying, tenant_slug character varying, plan character varying, billing_cycle character varying, status character varying, expires_at timestamp with time zone, pending_payment_id uuid, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION billing.sp_list_tenant_subscriptions(character varying,integer,integer) IS 'Every business with its current plan and its notice under review, by name, paged with total_count (BILLING-002 D3)';
