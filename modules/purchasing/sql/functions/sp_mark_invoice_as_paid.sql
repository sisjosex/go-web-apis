CREATE OR REPLACE FUNCTION purchasing.sp_mark_invoice_as_paid(p_tenant_id uuid, p_invoice_id uuid)
 RETURNS TABLE(id uuid, purchase_order_id uuid, invoice_number character varying, invoice_date date, invoice_amount numeric, tax_amount numeric, due_date date, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate invoice's PO belongs to tenant
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_order_invoices inv
        JOIN purchasing.purchase_orders po ON po.id = inv.purchase_order_id
        WHERE inv.id = p_invoice_id AND po.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'invoice.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE purchasing.purchase_order_invoices
    SET status = 'paid', updated_at = CURRENT_TIMESTAMP
    WHERE id = p_invoice_id
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
