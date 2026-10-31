CREATE OR REPLACE FUNCTION inventory.sp_close_count(p_tenant_id uuid, p_count_id uuid, p_closed_by uuid DEFAULT NULL::uuid)
 RETURNS character varying
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count  inventory.stock_counts%ROWTYPE;
    v_line   RECORD;
    v_number VARCHAR;
BEGIN
    SELECT sc.* INTO v_count
    FROM inventory.stock_counts sc
    WHERE sc.id = p_count_id AND sc.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_count.id IS NULL THEN
        RAISE EXCEPTION 'inventory.count.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_count.status NOT IN ('open', 'review') THEN
        RAISE EXCEPTION 'inventory.count.invalid-state' USING ERRCODE = 'P0001';
    END IF;
    IF EXISTS (SELECT 1 FROM inventory.stock_count_lines l WHERE l.count_id = p_count_id AND l.counted IS NULL) THEN
        RAISE EXCEPTION 'inventory.count.incomplete' USING ERRCODE = 'P0001';
    END IF;

    v_number := core.fn_next_document_number(p_tenant_id, 'inventory.adjustment', 'A', 3);
    FOR v_line IN
        SELECT l.product_id, l.sku_id, l.counted - l.expected AS difference,
               ps.current_quantity - ps.reserved_quantity AS available
        FROM inventory.stock_count_lines l
        LEFT JOIN inventory.product_stock ps ON ps.sku_id = l.sku_id AND ps.warehouse_id = v_count.warehouse_id
        WHERE l.count_id = p_count_id AND l.counted <> l.expected
    LOOP
        -- D1: the difference applies over today's stock. What sales already took cannot be missing twice.
        CONTINUE WHEN v_line.difference < 0 AND COALESCE(v_line.available, 0) <= 0;
        PERFORM inventory.sp_record_movement(
            p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'ADJUSTMENT',
            p_quantity := CASE WHEN v_line.difference < 0 THEN LEAST(-v_line.difference, v_line.available) ELSE v_line.difference END,
            p_reference_type := 'stock_count', p_reference_id := p_count_id,
            p_notes := CASE WHEN v_line.difference < 0 THEN 'faltante' ELSE 'sobrante' END,
            p_created_by := p_closed_by, p_sku_id := v_line.sku_id,
            p_direction := CASE WHEN v_line.difference < 0 THEN 'OUT' ELSE 'IN' END,
            p_reference_label := 'Conteo #' || v_count.number, p_warehouse_id := v_count.warehouse_id
        );
    END LOOP;

    UPDATE inventory.stock_counts sc
    SET status = 'closed', adjustment_number = v_number, closed_at = now(), closed_by = p_closed_by
    WHERE sc.id = p_count_id;
    RETURN v_number;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_close_count(uuid,uuid,uuid) IS 'Closes a count into one ADJUSTMENT per line with a difference and answers the adjustment number (INV-026). Raises inventory.count.not-found, inventory.count.invalid-state, inventory.count.incomplete.';
