CREATE OR REPLACE FUNCTION inventory.fn_sku_unit_cost(p_tenant_id uuid, p_sku_id uuid)
 RETURNS numeric
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_cost NUMERIC;
BEGIN
    SELECT SUM(b.current_quantity * b.unit_cost) / NULLIF(SUM(b.current_quantity), 0)
    INTO v_cost
    FROM inventory.product_batches b
    WHERE b.tenant_id = p_tenant_id AND b.sku_id = p_sku_id AND b.status <> 'void' AND b.current_quantity > 0;
    IF v_cost IS NOT NULL THEN
        RETURN v_cost;
    END IF;

    SELECT SUM(m.quantity * m.unit_cost) / NULLIF(SUM(m.quantity), 0)
    INTO v_cost
    FROM inventory.inventory_movements m
    WHERE m.tenant_id = p_tenant_id AND m.sku_id = p_sku_id AND m.quantity > 0 AND m.unit_cost IS NOT NULL;
    RETURN v_cost;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_sku_unit_cost(uuid,uuid) IS 'What one unit of a combination cost: weighted average of its active lots with units, else of its entries with cost, else NULL (INV-022 D1).';
