CREATE OR REPLACE FUNCTION inventory.sp_receive_transfer(p_tenant_id uuid, p_transfer_id uuid, p_lines jsonb DEFAULT NULL::jsonb, p_received_by uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_transfer inventory.transfers%ROWTYPE;
    v_line     RECORD;
    v_received NUMERIC;
    v_lot      RECORD;
    v_target   UUID;
    v_short    NUMERIC;
    v_off      NUMERIC;
BEGIN
    SELECT t.* INTO v_transfer
    FROM inventory.transfers t
    WHERE t.id = p_transfer_id AND t.tenant_id = p_tenant_id
    FOR UPDATE;
    IF v_transfer.id IS NULL THEN
        RAISE EXCEPTION 'inventory.transfer.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_transfer.status <> 'in_transit' THEN
        RAISE EXCEPTION 'inventory.transfer.invalid-state' USING ERRCODE = 'P0001';
    END IF;

    FOR v_line IN SELECT l.* FROM inventory.transfer_lines l WHERE l.transfer_id = p_transfer_id LOOP
        SELECT (e ->> 'received_quantity')::NUMERIC INTO v_received
        FROM jsonb_array_elements(CASE WHEN jsonb_typeof(p_lines) = 'array' THEN p_lines ELSE '[]'::jsonb END) e
        WHERE (e ->> 'sku_id')::UUID = v_line.sku_id;
        v_received := COALESCE(v_received, v_line.quantity);
        IF v_received < 0 OR v_received > v_line.quantity THEN
            RAISE EXCEPTION 'inventory.transfer.invalid-quantity' USING ERRCODE = 'P0001';
        END IF;

        -- All of it enters, and what did not arrive leaves as loss with the transfer's number (D3).
        PERFORM inventory.sp_record_movement(
            p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'TRANSFER',
            p_quantity := v_line.quantity, p_reference_type := 'transfer', p_reference_id := p_transfer_id,
            p_created_by := p_received_by, p_sku_id := v_line.sku_id, p_direction := 'IN',
            p_reference_label := 'Traspaso #' || v_transfer.number, p_warehouse_id := v_transfer.to_warehouse_id
        );
        -- INV-027: each lot taken lands on the destination's lot of the same number, expiry and cost, or opens it. The
        -- shortfall comes off the last lot taken and back.
        v_short := v_line.quantity - v_received;
        FOR v_lot IN
            SELECT b.quantity, s.lot_number, s.purchase_date, s.expiry_date, s.unit_cost, s.status
            FROM inventory.transfer_line_batches b
            INNER JOIN inventory.product_batches s ON s.id = b.source_batch_id
            WHERE b.transfer_line_id = v_line.id
            ORDER BY b.position DESC
        LOOP
            SELECT pb.id INTO v_target
            FROM inventory.product_batches pb
            WHERE pb.tenant_id = p_tenant_id AND pb.sku_id = v_line.sku_id AND pb.warehouse_id = v_transfer.to_warehouse_id
              AND pb.lot_number = v_lot.lot_number AND pb.expiry_date = v_lot.expiry_date AND pb.unit_cost = v_lot.unit_cost
              AND pb.status <> 'void'
            LIMIT 1
            FOR UPDATE;
            IF NOT FOUND THEN
                INSERT INTO inventory.product_batches (
                    tenant_id, product_id, sku_id, warehouse_id, lot_number, purchase_date, expiry_date, unit_cost,
                    initial_quantity, current_quantity, status, created_by, updated_by
                ) VALUES (
                    p_tenant_id, v_line.product_id, v_line.sku_id, v_transfer.to_warehouse_id, v_lot.lot_number,
                    v_lot.purchase_date, v_lot.expiry_date, v_lot.unit_cost, v_lot.quantity, 0, 'active', p_received_by, p_received_by
                ) RETURNING id INTO v_target;
            END IF;
            UPDATE inventory.product_batches pb
            SET current_quantity = pb.current_quantity + v_lot.quantity, updated_at = now()
            WHERE pb.id = v_target;

            v_off := LEAST(v_short, v_lot.quantity);
            IF v_off > 0 THEN
                PERFORM inventory.sp_record_movement(
                    p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'WASTE',
                    p_quantity := v_off, p_reference_type := 'transfer', p_reference_id := p_transfer_id,
                    p_created_by := p_received_by, p_sku_id := v_line.sku_id,
                    p_reference_label := 'Traspaso #' || v_transfer.number, p_warehouse_id := v_transfer.to_warehouse_id,
                    p_batch_id := v_target
                );
                v_short := v_short - v_off;
            END IF;
        END LOOP;
        -- What travelled without a lot.
        IF v_short > 0 THEN
            PERFORM inventory.sp_record_movement(
                p_tenant_id := p_tenant_id, p_product_id := v_line.product_id, p_movement_type := 'WASTE',
                p_quantity := v_short, p_reference_type := 'transfer', p_reference_id := p_transfer_id,
                p_created_by := p_received_by, p_sku_id := v_line.sku_id,
                p_reference_label := 'Traspaso #' || v_transfer.number, p_warehouse_id := v_transfer.to_warehouse_id
            );
        END IF;
        UPDATE inventory.transfer_lines l SET received_quantity = v_received WHERE l.id = v_line.id;
    END LOOP;

    UPDATE inventory.transfers t SET status = 'received', received_at = now(), received_by = p_received_by WHERE t.id = p_transfer_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_receive_transfer(uuid,uuid,jsonb,uuid) IS 'Receives a transfer in transit at its destination, with what was confirmed; a shortfall is a WASTE with the transfer''s number (INV-025 D3). Raises inventory.transfer.not-found, inventory.transfer.invalid-state, inventory.transfer.invalid-quantity.';
