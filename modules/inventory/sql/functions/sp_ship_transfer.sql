CREATE OR REPLACE FUNCTION inventory.sp_ship_transfer(p_tenant_id uuid, p_transfer_id uuid, p_carrier_note text DEFAULT NULL::text, p_shipped_by uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_transfer inventory.transfers%ROWTYPE;
    v_line     RECORD;
    v_lot      RECORD;
    v_left     NUMERIC;
    v_taken    NUMERIC;
    v_position INT;
BEGIN
    SELECT t.* INTO v_transfer
    FROM inventory.transfers t
    WHERE t.id = p_transfer_id AND t.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_transfer.id IS NULL THEN
        RAISE EXCEPTION 'inventory.transfer.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_transfer.status <> 'prepared' THEN
        RAISE EXCEPTION 'inventory.transfer.invalid-state' USING ERRCODE = 'P0001';
    END IF;

    FOR v_line IN SELECT l.* FROM inventory.transfer_lines l WHERE l.transfer_id = p_transfer_id LOOP
        PERFORM inventory.sp_release_reserved_stock(p_tenant_id, v_line.product_id, v_line.quantity, v_line.sku_id, p_shipped_by, v_transfer.from_warehouse_id);
        -- INV-027: first to expire, each lot its own movement; what the lots do not cover travels without a lot.
        v_left := v_line.quantity;
        v_position := 0;
        FOR v_lot IN
            SELECT pb.id, pb.current_quantity
            FROM inventory.product_batches pb
            WHERE pb.tenant_id = p_tenant_id AND pb.sku_id = v_line.sku_id AND pb.warehouse_id = v_transfer.from_warehouse_id
              AND pb.status <> 'void' AND pb.current_quantity > 0
            ORDER BY pb.expiry_date, pb.created_at
            FOR UPDATE
        LOOP
            EXIT WHEN v_left <= 0;
            v_taken := LEAST(v_left, v_lot.current_quantity);
            INSERT INTO inventory.transfer_line_batches (transfer_line_id, source_batch_id, quantity, position)
            VALUES (v_line.id, v_lot.id, v_taken, v_position);
            PERFORM inventory.sp_record_movement(
                p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'TRANSFER',
                p_quantity := v_taken, p_reference_type := 'transfer', p_reference_id := p_transfer_id,
                p_created_by := p_shipped_by, p_sku_id := v_line.sku_id, p_direction := 'OUT',
                p_reference_label := 'Traspaso #' || v_transfer.number, p_warehouse_id := v_transfer.from_warehouse_id,
                p_batch_id := v_lot.id
            );
            v_left := v_left - v_taken;
            v_position := v_position + 1;
        END LOOP;
        IF v_left > 0 THEN
            PERFORM inventory.sp_record_movement(
                p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'TRANSFER',
                p_quantity := v_left, p_reference_type := 'transfer', p_reference_id := p_transfer_id,
                p_created_by := p_shipped_by, p_sku_id := v_line.sku_id, p_direction := 'OUT',
                p_reference_label := 'Traspaso #' || v_transfer.number, p_warehouse_id := v_transfer.from_warehouse_id
            );
        END IF;
    END LOOP;

    UPDATE inventory.transfers t
    SET status = 'in_transit', carrier_note = p_carrier_note, shipped_at = now(), shipped_by = p_shipped_by
    WHERE t.id = p_transfer_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_ship_transfer(uuid,uuid,text,uuid) IS 'Marks a prepared transfer in transit: the units leave the origin, releasing their reservation, from its first lots to expire (INV-025 D3, INV-027). Raises inventory.transfer.not-found, inventory.transfer.invalid-state.';
