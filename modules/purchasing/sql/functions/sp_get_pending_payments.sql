CREATE OR REPLACE FUNCTION purchasing.sp_get_pending_payments(p_tenant_id uuid)
 RETURNS TABLE(id uuid, supplier_id uuid, po_number character varying, status character varying, order_date timestamp without time zone, expected_delivery_date date, total_amount numeric, paid_amount numeric, notes text, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT po.id, po.supplier_id, po.po_number, po.status, po.order_date, po.expected_delivery_date,
           po.total_amount, po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
    FROM purchasing.purchase_orders po
    JOIN (
        SELECT inv.purchase_order_id, MIN(inv.due_date) AS first_due
        FROM purchasing.purchase_order_invoices inv
        WHERE inv.status != 'paid'
        GROUP BY inv.purchase_order_id
    ) owed ON owed.purchase_order_id = po.id
    WHERE po.tenant_id = p_tenant_id
    ORDER BY owed.first_due ASC;
END;
$function$;
