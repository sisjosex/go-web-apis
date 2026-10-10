CREATE OR REPLACE FUNCTION purchasing.sp_add_invoice(p_tenant_id uuid, p_purchase_order_id uuid, p_invoice_number character varying, p_invoice_date date, p_invoice_amount numeric, p_tax_amount numeric, p_due_date date)
 RETURNS TABLE(id uuid, purchase_order_id uuid, invoice_number character varying, invoice_date date, invoice_amount numeric, tax_amount numeric, due_date date, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate PO exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_purchase_order_id AND po.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO purchasing.purchase_order_invoices(purchase_order_id, invoice_number, invoice_date, invoice_amount, tax_amount, due_date)
    VALUES(p_purchase_order_id, p_invoice_number, p_invoice_date, p_invoice_amount, COALESCE(p_tax_amount, 0), p_due_date)
    RETURNING
        purchase_order_invoices.id,
        purchase_order_invoices.purchase_order_id,
        purchase_order_invoices.invoice_number,
        purchase_order_invoices.invoice_date,
        purchase_order_invoices.invoice_amount,
        purchase_order_invoices.tax_amount,
        purchase_order_invoices.due_date,
        purchase_order_invoices.status,
        purchase_order_invoices.created_at,
        purchase_order_invoices.updated_at;
END;
$function$;
