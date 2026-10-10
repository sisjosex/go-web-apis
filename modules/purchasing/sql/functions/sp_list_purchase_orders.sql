CREATE OR REPLACE FUNCTION purchasing.sp_list_purchase_orders(p_tenant_id uuid, p_status character varying, p_page_size integer, p_offset integer)
 RETURNS TABLE(id uuid, supplier_id uuid, po_number character varying, status character varying, order_date timestamp without time zone, expected_delivery_date date, total_amount numeric, paid_amount numeric, notes text, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT po.id, po.supplier_id, po.po_number, po.status, po.order_date, po.expected_delivery_date,
           po.total_amount, po.paid_amount, po.notes, po.created_by, po.created_at, po.updated_at
    FROM purchasing.purchase_orders po
    WHERE po.tenant_id = p_tenant_id AND (p_status IS NULL OR po.status = p_status)
    ORDER BY po.created_at DESC
    LIMIT p_page_size OFFSET p_offset;
END;
$function$;
