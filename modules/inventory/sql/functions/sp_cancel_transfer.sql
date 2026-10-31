CREATE OR REPLACE FUNCTION inventory.sp_cancel_transfer(p_tenant_id uuid, p_transfer_id uuid, p_cancelled_by uuid DEFAULT NULL::uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_transfer inventory.transfers%ROWTYPE;
    v_line     RECORD;
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
        PERFORM inventory.sp_release_reserved_stock(p_tenant_id, v_line.product_id, v_line.quantity, v_line.sku_id, p_cancelled_by, v_transfer.from_warehouse_id);
    END LOOP;
    UPDATE inventory.transfers t SET status = 'cancelled' WHERE t.id = p_transfer_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_cancel_transfer(uuid,uuid,uuid) IS 'Cancels a transfer still prepared, releasing its reservation (INV-025 D3). Raises inventory.transfer.not-found, inventory.transfer.invalid-state.';
