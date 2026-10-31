CREATE OR REPLACE FUNCTION inventory.fn_resolve_warehouse(p_tenant_id uuid, p_warehouse_id uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_warehouse_id IS NULL THEN
        RETURN inventory.fn_default_warehouse(p_tenant_id);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM inventory.warehouses w
        WHERE w.id = p_warehouse_id AND w.tenant_id = p_tenant_id AND w.status = 'active'
    ) THEN
        RAISE EXCEPTION 'inventory.warehouse.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN p_warehouse_id;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_resolve_warehouse(uuid,uuid) IS 'The warehouse a stock write uses: the one named, active and of this business, or the default. Raises inventory.warehouse.not-found.';
