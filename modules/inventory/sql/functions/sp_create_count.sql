CREATE OR REPLACE FUNCTION inventory.sp_create_count(p_tenant_id uuid, p_warehouse_id uuid, p_category_id uuid, p_blind boolean, p_created_by uuid DEFAULT NULL::uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id        UUID;
    v_warehouse UUID := inventory.fn_resolve_warehouse(p_tenant_id, p_warehouse_id);
BEGIN
    IF p_category_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM inventory.product_categories c WHERE c.id = p_category_id AND c.tenant_id = p_tenant_id AND c.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    BEGIN
        INSERT INTO inventory.stock_counts (tenant_id, number, warehouse_id, category_id, blind, created_by)
        VALUES (p_tenant_id, core.fn_next_document_number(p_tenant_id, 'inventory.count', 'C', 3), v_warehouse, p_category_id, COALESCE(p_blind, FALSE), p_created_by)
        RETURNING id INTO v_id;
    EXCEPTION WHEN unique_violation THEN
        RAISE EXCEPTION 'inventory.count.already-open' USING ERRCODE = 'P0001';
    END;

    -- One line per active combination of the scope in one insert, so what nobody counts is visible (D3).
    INSERT INTO inventory.stock_count_lines (count_id, product_id, sku_id)
    SELECT v_id, s.product_id, s.id
    FROM inventory.product_stock ps
    INNER JOIN inventory.product_skus s ON s.id = ps.sku_id AND s.status = 'active'
    INNER JOIN inventory.products p ON p.id = s.product_id AND p.status = 'active'
    WHERE ps.warehouse_id = v_warehouse AND s.tenant_id = p_tenant_id
      AND (NOT s.is_default OR NOT EXISTS (SELECT 1 FROM inventory.product_skus o WHERE o.product_id = s.product_id AND NOT o.is_default))
      AND (p_category_id IS NULL OR EXISTS (
          SELECT 1 FROM inventory.product_category_mapping m WHERE m.product_id = s.product_id AND m.category_id = p_category_id
      ));
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_create_count(uuid,uuid,uuid,boolean,uuid) IS 'Starts a count of a warehouse and optional category, with a line per active combination (INV-026 D3). Raises inventory.warehouse.not-found, category.not-found, inventory.count.already-open.';
