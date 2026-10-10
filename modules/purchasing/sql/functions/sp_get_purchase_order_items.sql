CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_order_items(p_tenant_id uuid, p_purchase_order_id uuid)
 RETURNS TABLE(id uuid, purchase_order_id uuid, product_id uuid, quantity integer, unit_cost numeric, line_total numeric, received_quantity integer, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Validate PO belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM purchasing.purchase_orders po WHERE po.id = p_purchase_order_id AND po.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'po.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        poi.id,
        poi.purchase_order_id,
        poi.product_id,
        poi.quantity,
        poi.unit_cost,
        poi.line_total,
        poi.received_quantity,
        poi.status,
        poi.created_at,
        poi.updated_at
    FROM purchasing.purchase_order_items poi
    WHERE poi.purchase_order_id = p_purchase_order_id
    ORDER BY poi.created_at;
END;
$function$;
