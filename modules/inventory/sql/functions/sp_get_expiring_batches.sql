CREATE OR REPLACE FUNCTION inventory.sp_get_expiring_batches(p_tenant_id uuid, p_today date, p_warning_days integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_warehouse_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, product_id uuid, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, days_to_expiry integer, product_name character varying, sku_id uuid, sku character varying, total_count bigint, warehouse_id uuid, warehouse_name character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY SELECT
        pb.id,
        pb.product_id,
        CAST(pb.lot_number AS VARCHAR),
        pb.purchase_date,
        pb.expiry_date,
        pb.unit_cost,
        pb.initial_quantity,
        pb.current_quantity,
        CAST(inventory.fn_batch_status(pb.expiry_date, pb.status, p_today) AS VARCHAR),
        CAST(pb.expiry_date - p_today AS INT),
        CAST(p.name AS VARCHAR),
        pb.sku_id,
        CAST(s.sku AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT),
        pb.warehouse_id,
        CAST(w.name AS VARCHAR)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    INNER JOIN inventory.warehouses w
        ON w.id = pb.warehouse_id
    WHERE pb.tenant_id = p_tenant_id
        AND (p_warehouse_id IS NULL OR pb.warehouse_id = p_warehouse_id)
        -- A lot is still good on its expiry date (INV-018 D2), so the window starts today; a
        -- range on the column so idx_product_batches_tenant_open_expiry can serve it.
        AND pb.expiry_date >= p_today
        AND pb.expiry_date <= p_today + p_warning_days
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC, pb.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_expiring_batches(uuid,date,integer,integer,integer,uuid) IS 'One page of the lots expiring within the warning window, soonest first, with the product name, the combination and the total_count. A voided lot never appears: the status filter excludes anything not active or expiring';
