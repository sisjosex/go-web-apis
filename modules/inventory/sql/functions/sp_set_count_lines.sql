CREATE OR REPLACE FUNCTION inventory.sp_set_count_lines(p_tenant_id uuid, p_count_id uuid, p_lines jsonb)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count inventory.stock_counts%ROWTYPE;
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

    -- D1: expected is the stock when the line is first counted, so a sale after it is not read as missing.
    UPDATE inventory.stock_count_lines l
    SET counted = (e ->> 'counted')::NUMERIC,
        expected = COALESCE(l.expected, inventory.fn_warehouse_quantity(l.sku_id, v_count.warehouse_id)),
        counted_at = now()
    FROM jsonb_array_elements(p_lines) e
    WHERE l.count_id = p_count_id AND l.sku_id = (e ->> 'sku_id')::UUID;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_count_lines(uuid,uuid,jsonb) IS 'Sets what was counted on several lines; the first count of a line keeps the stock of that moment (INV-026 D1). Raises inventory.count.not-found, inventory.count.invalid-state.';
