CREATE OR REPLACE FUNCTION billing.sp_record_payment(p_subscription_id uuid, p_user_id uuid, p_amount numeric, p_currency character varying DEFAULT 'USD'::character varying, p_status character varying DEFAULT 'completed'::character varying, p_provider character varying DEFAULT NULL::character varying, p_provider_payment_id character varying DEFAULT NULL::character varying, p_period_start timestamp with time zone DEFAULT NULL::timestamp with time zone, p_period_end timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, subscription_id uuid, user_id uuid, amount numeric, currency character varying, status character varying, provider character varying, provider_payment_id character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate subscription belongs to user
    IF NOT EXISTS (
        SELECT 1 FROM billing.subscriptions s
        WHERE s.id = p_subscription_id AND s.user_id = p_user_id
    ) THEN
        RAISE EXCEPTION 'billing.payment.subscription-not-found'
            USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO billing.payments (
        subscription_id, user_id, amount, currency, status,
        provider, provider_payment_id,
        paid_at, period_start, period_end
    )
    VALUES (
        p_subscription_id, p_user_id, p_amount, p_currency, p_status,
        p_provider, p_provider_payment_id,
        CASE WHEN p_status = 'completed' THEN NOW() ELSE NULL END,
        p_period_start, p_period_end
    )
    RETURNING
        billing.payments.id,
        billing.payments.subscription_id,
        billing.payments.user_id,
        billing.payments.amount,
        CAST(billing.payments.currency            AS VARCHAR),
        CAST(billing.payments.status              AS VARCHAR),
        CAST(billing.payments.provider            AS VARCHAR),
        CAST(billing.payments.provider_payment_id AS VARCHAR),
        billing.payments.paid_at,
        billing.payments.period_start,
        billing.payments.period_end,
        billing.payments.created_at;
END;
$function$;
