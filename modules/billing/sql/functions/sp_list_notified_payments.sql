CREATE OR REPLACE FUNCTION billing.sp_list_notified_payments()
 RETURNS TABLE(id uuid, tenant_id uuid, tenant_name character varying, user_id uuid, plan character varying, billing_cycle character varying, amount numeric, currency character varying, reference character varying, created_at timestamp with time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT p.id, p.tenant_id, CAST(t.name AS VARCHAR), p.user_id, CAST(p.plan AS VARCHAR),
           CAST(p.billing_cycle AS VARCHAR), p.amount, CAST(p.currency AS VARCHAR), CAST(p.reference AS VARCHAR),
           p.created_at
    FROM billing.payments p
    INNER JOIN tenancy.tenants t ON t.id = p.tenant_id
    WHERE p.status = 'notified'
    ORDER BY p.created_at;
END;
$function$;
