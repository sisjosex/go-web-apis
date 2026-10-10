CREATE OR REPLACE FUNCTION purchasing.sp_get_invoices(p_tenant_id uuid, p_purchase_order_id uuid)
 RETURNS TABLE(id uuid, purchase_order_id uuid, invoice_number character varying, invoice_date date, invoice_amount numeric, tax_amount numeric, due_date date, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate PO belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_purchase_order_id AND po.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        inv.id,
        inv.purchase_order_id,
        inv.invoice_number,
        inv.invoice_date,
        inv.invoice_amount,
        inv.tax_amount,
        inv.due_date,
        inv.status,
        inv.created_at,
        inv.updated_at
    FROM purchasing.purchase_order_invoices inv
    WHERE inv.purchase_order_id = p_purchase_order_id
    ORDER BY inv.invoice_date DESC;
END;
$function$;
