CREATE OR REPLACE FUNCTION billing.sp_payment_contact(p_payment_id uuid)
 RETURNS TABLE(email character varying, first_name character varying, tenant_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    RETURN QUERY
    SELECT CAST(u.email AS VARCHAR), CAST(u.first_name AS VARCHAR), CAST(t.name AS VARCHAR)
    FROM billing.payments p
    LEFT JOIN auth.users u ON u.id = p.user_id
    LEFT JOIN tenancy.tenants t ON t.id = p.tenant_id
    WHERE p.id = p_payment_id;
END;
$function$;

COMMENT ON FUNCTION billing.sp_payment_contact(uuid) IS 'The notifier''s email and name and the business''s name, for the payment emails (BILLING-002 D4)';
