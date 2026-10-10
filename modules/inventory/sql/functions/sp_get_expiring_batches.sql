CREATE OR REPLACE FUNCTION inventory.sp_get_expiring_batches(p_tenant_id uuid, p_warning_days integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, product_id uuid, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, days_to_expiry integer, product_name character varying, sku_id uuid, sku character varying, total_count bigint)
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
        CAST(pb.status AS VARCHAR),
        CAST(pb.expiry_date - CURRENT_DATE AS INT),
        CAST(p.name AS VARCHAR),
        pb.sku_id,
        CAST(s.sku AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.products p
        ON p.id = pb.product_id
        AND p.tenant_id = pb.tenant_id
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        -- Same window as before (1..n days out, never what already expired), as a range
        -- on the column so idx_product_batches_tenant_open_expiry can serve it.
        AND pb.expiry_date >  CURRENT_DATE
        AND pb.expiry_date <= CURRENT_DATE + p_warning_days
        AND pb.status IN ('active', 'expiring_soon')
    ORDER BY pb.expiry_date ASC, pb.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_expiring_batches(uuid,integer,integer,integer) IS 'One page of the lots expiring within the warning window, soonest first, with the product name, the combination and the total_count. A voided lot never appears: the status filter excludes anything not active or expiring';
