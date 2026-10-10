CREATE OR REPLACE FUNCTION billing.sp_get_subscription(p_user_id uuid)
 RETURNS TABLE(id uuid, user_id uuid, plan character varying, status character varying, started_at timestamp with time zone, expires_at timestamp with time zone, canceled_at timestamp with time zone, stripe_sub_id character varying, stripe_customer_id character varying, created_at timestamp with time zone, updated_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        s.id,
        s.user_id,
        CAST(s.plan AS VARCHAR),
        CAST(s.status AS VARCHAR),
        s.started_at,
        s.expires_at,
        s.canceled_at,
        CAST(s.stripe_sub_id AS VARCHAR),
        CAST(s.stripe_customer_id AS VARCHAR),
        s.created_at,
        s.updated_at
    FROM billing.subscriptions s
    WHERE s.user_id = p_user_id
    ORDER BY
        CASE WHEN s.status IN ('active', 'trial') THEN 0 ELSE 1 END,
        s.created_at DESC
    LIMIT 1;
END;
$function$;
