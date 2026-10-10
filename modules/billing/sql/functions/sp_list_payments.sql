CREATE OR REPLACE FUNCTION billing.sp_list_payments(p_user_id uuid, p_page integer DEFAULT 1, p_limit integer DEFAULT 10)
 RETURNS TABLE(id uuid, subscription_id uuid, amount numeric, currency character varying, status character varying, provider character varying, provider_payment_id character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        p.id,
        p.subscription_id,
        p.amount,
        CAST(p.currency            AS VARCHAR),
        CAST(p.status              AS VARCHAR),
        CAST(p.provider            AS VARCHAR),
        CAST(p.provider_payment_id AS VARCHAR),
        p.paid_at,
        p.period_start,
        p.period_end,
        p.created_at,
        COUNT(*) OVER() AS total_count
    FROM billing.payments p
    WHERE p.user_id = p_user_id
    ORDER BY p.created_at DESC
    OFFSET (p_page - 1) * p_limit
    LIMIT p_limit;
END;
$function$;
