CREATE OR REPLACE FUNCTION inventory.sp_list_batches_by_product(p_tenant_id uuid, p_product_id uuid, p_only_active boolean DEFAULT true, p_sku_id uuid DEFAULT NULL::uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, product_id uuid, lot_number character varying, purchase_date date, expiry_date date, unit_cost numeric, initial_quantity numeric, current_quantity numeric, status character varying, days_to_expiry integer, sku_id uuid, sku character varying, total_count bigint)
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
        pb.sku_id,
        CAST(s.sku AS VARCHAR),
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM inventory.product_batches pb
    INNER JOIN inventory.product_skus s
        ON s.id = pb.sku_id
    WHERE pb.tenant_id = p_tenant_id
        AND pb.product_id = p_product_id
        AND (p_sku_id IS NULL OR pb.sku_id = p_sku_id)
        AND (NOT p_only_active OR pb.status NOT IN ('expired', 'void'))
    -- FIFO order; pb.id breaks ties so a page boundary never splits a tie differently twice.
    ORDER BY pb.expiry_date ASC, pb.created_at ASC, pb.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_batches_by_product(uuid,uuid,boolean,uuid,integer,integer) IS 'One page of a product''s lots in FIFO order, each with its combination and the total_count the filters match. p_sku_id narrows to one combination; p_only_active hides expired and voided lots';
