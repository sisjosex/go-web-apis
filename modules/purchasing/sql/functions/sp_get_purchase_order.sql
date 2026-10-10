CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_order(p_po_id uuid, p_tenant_id uuid)
 RETURNS TABLE(id uuid, supplier_id uuid, po_number character varying, status character varying, order_date timestamp without time zone, expected_delivery_date date, total_amount numeric, paid_amount numeric, notes text, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.purchase_orders po
        WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        po.id,
        po.supplier_id,
        po.po_number,
        po.status,
        po.order_date,
        po.expected_delivery_date,
        po.total_amount,
        po.paid_amount,
        po.notes,
        po.created_by,
        po.created_at,
        po.updated_at
    FROM purchasing.purchase_orders po
    WHERE po.id = p_po_id AND po.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_purchase_order(uuid,uuid) IS 'Retrieves a single purchase order by ID scoped to a tenant. Raises po.not-found if not found.';
