CREATE OR REPLACE FUNCTION inventory.sp_set_warehouse_status(p_tenant_id uuid, p_warehouse_id uuid, p_status character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_default BOOLEAN;
BEGIN
    SELECT w.is_default INTO v_default FROM inventory.warehouses w WHERE w.id = p_warehouse_id AND w.tenant_id = p_tenant_id;
    IF v_default IS NULL THEN
        RAISE EXCEPTION 'inventory.warehouse.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF p_status = 'archived' THEN
        IF v_default THEN
            RAISE EXCEPTION 'inventory.warehouse.default' USING ERRCODE = 'P0001';
        END IF;
        IF EXISTS (
            SELECT 1 FROM inventory.product_stock ps
            WHERE ps.warehouse_id = p_warehouse_id AND (ps.current_quantity > 0 OR ps.reserved_quantity > 0)
        ) THEN
            RAISE EXCEPTION 'inventory.warehouse.has-stock' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    UPDATE inventory.warehouses w SET status = p_status, updated_at = now() WHERE w.id = p_warehouse_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_warehouse_status(uuid,uuid,character varying) IS 'Archives or reactivates a warehouse. Raises inventory.warehouse.not-found, inventory.warehouse.default, inventory.warehouse.has-stock.';
