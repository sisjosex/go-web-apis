CREATE OR REPLACE FUNCTION sales.sp_delete_customer(p_tenant_id uuid, p_customer_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.customers c WHERE c.id = p_customer_id AND c.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.customers SET status = 'inactive' WHERE id = p_customer_id AND tenant_id = p_tenant_id;
END;
$function$;
