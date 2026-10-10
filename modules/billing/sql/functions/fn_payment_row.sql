CREATE OR REPLACE FUNCTION billing.fn_payment_row(p_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, user_id uuid, plan character varying, billing_cycle character varying, amount numeric, currency character varying, status character varying, reference character varying, paid_at timestamp with time zone, period_start timestamp with time zone, period_end timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT p.id, p.tenant_id, p.user_id, CAST(p.plan AS VARCHAR), CAST(p.billing_cycle AS VARCHAR), p.amount,
           CAST(p.currency AS VARCHAR), CAST(p.status AS VARCHAR), CAST(p.reference AS VARCHAR), p.paid_at,
           p.period_start, p.period_end, p.created_at
    FROM billing.payments p WHERE p.id = p_id;
$function$;
