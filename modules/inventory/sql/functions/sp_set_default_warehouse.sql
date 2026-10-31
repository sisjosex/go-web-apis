CREATE OR REPLACE FUNCTION inventory.sp_set_default_warehouse(p_tenant_id uuid, p_warehouse_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM inventory.warehouses w
        WHERE w.id = p_warehouse_id AND w.tenant_id = p_tenant_id AND w.status = 'active'
    ) THEN
        RAISE EXCEPTION 'inventory.warehouse.not-found' USING ERRCODE = 'P0001';
    END IF;
    UPDATE inventory.warehouses w SET is_default = FALSE, updated_at = now() WHERE w.tenant_id = p_tenant_id AND w.is_default;
    UPDATE inventory.warehouses w SET is_default = TRUE, updated_at = now() WHERE w.id = p_warehouse_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_default_warehouse(uuid,uuid) IS 'Moves the default mark to an active warehouse of the business. Raises inventory.warehouse.not-found.';
