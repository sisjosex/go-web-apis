CREATE OR REPLACE FUNCTION inventory.sp_scan_count(p_tenant_id uuid, p_count_id uuid, p_code character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count   inventory.stock_counts%ROWTYPE;
    v_sku     UUID;
    v_product UUID;
    v_factor  NUMERIC := 1;
BEGIN
    SELECT sc.* INTO v_count
    FROM inventory.stock_counts sc
    WHERE sc.id = p_count_id AND sc.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_count.id IS NULL THEN
        RAISE EXCEPTION 'inventory.count.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_count.status <> 'open' THEN
        RAISE EXCEPTION 'inventory.count.invalid-state' USING ERRCODE = 'P0001';
    END IF;

    -- A presentation's barcode counts what it holds: a box of 12 is 12.
    SELECT sc.sku_id, pu.factor INTO v_sku, v_factor
    FROM inventory.sku_codes sc
    INNER JOIN inventory.product_units pu ON pu.id = sc.product_unit_id
    WHERE sc.tenant_id = p_tenant_id AND sc.code = TRIM(p_code);
    IF v_sku IS NULL THEN
        v_factor := 1;
        SELECT s.id INTO v_sku FROM inventory.product_skus s WHERE s.tenant_id = p_tenant_id AND s.sku = TRIM(p_code);
    END IF;
    SELECT s.product_id INTO v_product
    FROM inventory.product_skus s
    INNER JOIN inventory.product_stock ps ON ps.sku_id = s.id AND ps.warehouse_id = v_count.warehouse_id
    WHERE s.id = v_sku;
    IF v_product IS NULL THEN
        RAISE EXCEPTION 'inventory.code.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO inventory.stock_count_lines AS l (count_id, product_id, sku_id, expected, counted, counted_at)
    VALUES (p_count_id, v_product, v_sku, inventory.fn_warehouse_quantity(v_sku, v_count.warehouse_id), v_factor, now())
    ON CONFLICT (count_id, sku_id) DO UPDATE
    SET counted = COALESCE(l.counted, 0) + v_factor,
        expected = COALESCE(l.expected, EXCLUDED.expected),
        counted_at = now();
END;
$function$;

COMMENT ON FUNCTION inventory.sp_scan_count(uuid,uuid,character varying) IS 'One scan on a count: its line goes up by what the code holds; a combination of the warehouse outside the scope joins the count. Raises inventory.code.not-found, inventory.count.not-found, inventory.count.invalid-state.';
