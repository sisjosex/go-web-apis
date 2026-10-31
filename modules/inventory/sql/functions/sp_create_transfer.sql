CREATE OR REPLACE FUNCTION inventory.sp_create_transfer(p_tenant_id uuid, p_from_warehouse_id uuid, p_to_warehouse_id uuid, p_lines jsonb, p_notes text DEFAULT NULL::text, p_created_by uuid DEFAULT NULL::uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id      UUID;
    v_line    JSONB;
    v_sku_id  UUID;
    v_product UUID;
    v_qty     NUMERIC;
BEGIN
    IF p_from_warehouse_id = p_to_warehouse_id THEN
        RAISE EXCEPTION 'inventory.transfer.same-warehouse' USING ERRCODE = 'P0001';
    END IF;
    PERFORM inventory.fn_resolve_warehouse(p_tenant_id, p_from_warehouse_id);
    PERFORM inventory.fn_resolve_warehouse(p_tenant_id, p_to_warehouse_id);

    INSERT INTO inventory.transfers (tenant_id, number, from_warehouse_id, to_warehouse_id, notes, prepared_by)
    VALUES (p_tenant_id, core.fn_next_document_number(p_tenant_id, 'inventory.transfer', 'T', 3), p_from_warehouse_id, p_to_warehouse_id, p_notes, p_created_by)
    RETURNING id INTO v_id;

    FOR v_line IN SELECT * FROM jsonb_array_elements(p_lines) LOOP
        v_sku_id := (v_line ->> 'sku_id')::UUID;
        v_qty := (v_line ->> 'quantity')::NUMERIC;
        SELECT s.product_id INTO v_product FROM inventory.product_skus s WHERE s.id = v_sku_id AND s.tenant_id = p_tenant_id;
        IF v_product IS NULL THEN
            RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
        END IF;
        INSERT INTO inventory.transfer_lines (transfer_id, product_id, sku_id, quantity) VALUES (v_id, v_product, v_sku_id, v_qty);
        -- D3: what is prepared cannot be sold at the origin.
        PERFORM inventory.sp_reserve_stock_for_order(p_tenant_id, v_product, v_qty, v_sku_id, p_created_by, p_from_warehouse_id);
    END LOOP;
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_create_transfer(uuid,uuid,uuid,jsonb,text,uuid) IS 'Prepares a transfer with its number and reserves its lines at the origin (INV-025 D3). Raises inventory.transfer.same-warehouse, inventory.warehouse.not-found, inventory.sku.not-found, inventory.insufficient-stock.';
