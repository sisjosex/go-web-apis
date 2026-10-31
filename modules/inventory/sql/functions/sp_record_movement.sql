CREATE OR REPLACE FUNCTION inventory.sp_record_movement(p_tenant_id uuid, p_product_id uuid, p_movement_type character varying, p_quantity numeric, p_reference_type character varying DEFAULT NULL::character varying, p_reference_id uuid DEFAULT NULL::uuid, p_unit_cost numeric DEFAULT NULL::numeric, p_notes text DEFAULT NULL::text, p_created_by uuid DEFAULT NULL::uuid, p_sku_id uuid DEFAULT NULL::uuid, p_direction character varying DEFAULT NULL::character varying, p_reference_label character varying DEFAULT NULL::character varying, p_warehouse_id uuid DEFAULT NULL::uuid, p_batch_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(movement_id uuid, new_stock_quantity numeric, message text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_id        UUID;
    v_movement_id   UUID;
    v_new_stock     DECIMAL;
    v_current_stock DECIMAL;
    v_reserved      DECIMAL;
    v_reorder_level DECIMAL;
    v_status        VARCHAR;
    v_sign          INTEGER;
    v_signed_qty    DECIMAL;
    v_unit_cost     DECIMAL;
    v_warehouse_id  UUID;
    v_left          DECIMAL;
    v_lot           RECORD;
    v_taken         DECIMAL;
BEGIN
    -- INV-011 D1 — the combination is required, never defaulted.
    IF p_sku_id IS NULL AND inventory.fn_requires_sku(p_tenant_id, p_product_id) THEN
        RAISE EXCEPTION 'inventory.movement.sku-required' USING ERRCODE = 'P0001';
    END IF;

    -- Validates the product against the tenant and resolves NULL to the default
    v_sku_id := inventory.fn_resolve_sku(p_tenant_id, p_product_id, p_sku_id);

    -- Validate movement type before asking for its direction: an unknown type
    -- has no entry in the table and would come back as direction-required,
    -- which names the wrong problem.
    IF p_movement_type NOT IN ('PURCHASE', 'SALE', 'ADJUSTMENT', 'TRANSFER', 'RETURN', 'WASTE', 'PRODUCTION') THEN
        RAISE EXCEPTION 'inventory.invalid-movement-type' USING ERRCODE = 'P0001';
    END IF;

    -- INV-013 D1 — the type decides the sign, the caller only supplies the
    -- magnitude. ABS() rather than a rejection of a negative: a client that
    -- still sends -5 for a SALE means the same movement it always meant.
    v_sign       := inventory.fn_movement_direction(p_movement_type, p_direction);
    v_signed_qty := v_sign * ABS(p_quantity);

    -- INV-013 D2/D3 — unit_cost is what was paid, so it is asked exactly where
    -- something was bought and stored nowhere else.
    IF p_movement_type IN ('PURCHASE', 'PRODUCTION') THEN
        IF p_unit_cost IS NULL OR p_unit_cost <= 0 THEN
            RAISE EXCEPTION 'inventory.movement.unit-cost-required'
                USING ERRCODE = 'P0001';
        END IF;
        v_unit_cost := p_unit_cost;
    ELSE
        v_unit_cost := NULL;
    END IF;

    v_warehouse_id := inventory.fn_resolve_warehouse(p_tenant_id, p_warehouse_id);

    -- Lock the row before reading it: without this two concurrent SALEs both
    -- pass the insufficient-stock check below.
    SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
    INTO v_current_stock, v_reserved, v_reorder_level
    FROM inventory.product_stock ps
    WHERE ps.sku_id = v_sku_id AND ps.warehouse_id = v_warehouse_id
    FOR UPDATE;

    -- INV-025: what enters a warehouse that never had this combination opens its row, with the minimum it has elsewhere.
    IF NOT FOUND AND v_sign > 0 AND EXISTS (SELECT 1 FROM inventory.product_stock ps WHERE ps.sku_id = v_sku_id) THEN
        INSERT INTO inventory.product_stock (product_id, sku_id, warehouse_id, current_quantity, reorder_level)
        SELECT p_product_id, v_sku_id, v_warehouse_id, 0, ps.reorder_level
        FROM inventory.product_stock ps
        WHERE ps.sku_id = v_sku_id
        ORDER BY ps.reorder_level DESC
        LIMIT 1
        ON CONFLICT (sku_id, warehouse_id) DO NOTHING;

        SELECT ps.current_quantity, ps.reserved_quantity, ps.reorder_level
        INTO v_current_stock, v_reserved, v_reorder_level
        FROM inventory.product_stock ps
        WHERE ps.sku_id = v_sku_id AND ps.warehouse_id = v_warehouse_id
        FOR UPDATE;
    END IF;

    -- Without this the arithmetic below is NULL, the guard passes and the movement
    -- is logged against stock that does not exist.
    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory.stock-not-found' USING ERRCODE = 'P0001';
    END IF;

    v_new_stock := v_current_stock + v_signed_qty;

    -- Every outbound type, not only SALE: WASTE and an ADJUSTMENT OUT drive the
    -- count negative just as readily. Only what is not reserved can leave (INV-018 D3):
    -- to sell a reserved unit, release the reservation first.
    IF v_sign < 0 AND v_new_stock < v_reserved THEN
        RAISE EXCEPTION 'inventory.insufficient-stock' USING ERRCODE = 'P0001';
    END IF;

    v_status := inventory.fn_stock_status(v_new_stock, v_reorder_level);

    -- Insert movement — the stored quantity carries the sign, so the audit trail
    -- reads as what happened without re-deriving it from the type.
    INSERT INTO inventory.inventory_movements (
        tenant_id, product_id, sku_id, movement_type, quantity,
        reference_type, reference_id, unit_cost, notes, created_by, reference_label, warehouse_id
    )
    VALUES (p_tenant_id, p_product_id, v_sku_id, p_movement_type, v_signed_qty,
            p_reference_type, p_reference_id, v_unit_cost, p_notes, p_created_by, p_reference_label, v_warehouse_id)
    RETURNING inventory_movements.id INTO v_movement_id;

    -- Update stock with new status
    UPDATE inventory.product_stock ps
    SET current_quantity = v_new_stock,
        status = v_status,
        last_updated_at = CURRENT_TIMESTAMP
    WHERE ps.sku_id = v_sku_id AND ps.warehouse_id = v_warehouse_id;

    -- INV-027: lots follow the stock, so they never read more than there is. What leaves comes off the lot named,
    -- then off the warehouse's first to expire; units beyond the lots come off stock only.
    IF p_batch_id IS NOT NULL AND v_sign > 0 THEN
        UPDATE inventory.product_batches pb
        SET current_quantity = pb.current_quantity + ABS(p_quantity), updated_at = CURRENT_TIMESTAMP
        WHERE pb.id = p_batch_id AND pb.tenant_id = p_tenant_id;
    ELSIF v_sign < 0 THEN
        v_left := ABS(p_quantity);
        FOR v_lot IN
            SELECT pb.id, pb.current_quantity
            FROM inventory.product_batches pb
            WHERE pb.tenant_id = p_tenant_id AND pb.sku_id = v_sku_id AND pb.current_quantity > 0
              AND (pb.id = p_batch_id OR (pb.warehouse_id = v_warehouse_id AND pb.status <> 'void'))
            ORDER BY (pb.id = p_batch_id) DESC NULLS LAST, pb.expiry_date, pb.created_at
            FOR UPDATE
        LOOP
            v_taken := LEAST(v_left, v_lot.current_quantity);
            UPDATE inventory.product_batches pb
            SET current_quantity = pb.current_quantity - v_taken, updated_at = CURRENT_TIMESTAMP
            WHERE pb.id = v_lot.id;
            v_left := v_left - v_taken;
            EXIT WHEN v_left <= 0;
        END LOOP;
    END IF;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_movement_id,
        v_new_stock,
        CAST('Movement recorded successfully' AS TEXT);
END;
$function$;

COMMENT ON FUNCTION inventory.sp_record_movement(uuid,uuid,character varying,numeric,character varying,uuid,numeric,text,uuid,uuid,character varying,character varying,uuid,uuid) IS 'Records an inventory movement against a SKU and updates that SKU''s quantities. p_quantity is a magnitude: the sign comes from inventory.fn_movement_direction(p_movement_type, p_direction) and is what gets stored, so a SALE reduces stock (INV-013 D1). p_unit_cost is required and > 0 on PURCHASE and PRODUCTION and stored NULL on every other type (INV-013 D2/D3). p_sku_id is required when the product tracks stock by variant and defaults to the product SKU otherwise. Takes the stock row FOR UPDATE so concurrent outbound movements cannot oversell; an outbound movement consumes only what is not reserved (INV-018 D3) and a SKU with no product_stock row raises inventory.stock-not-found.';
