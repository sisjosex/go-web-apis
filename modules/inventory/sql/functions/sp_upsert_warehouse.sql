CREATE OR REPLACE FUNCTION inventory.sp_upsert_warehouse(p_tenant_id uuid, p_warehouse_id uuid, p_name character varying, p_address character varying, p_city character varying, p_kind character varying, p_manager_user_id uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID := p_warehouse_id;
BEGIN
    IF v_id IS NULL THEN
        -- The first one a business creates comes after its Principal.
        PERFORM inventory.fn_default_warehouse(p_tenant_id);
        INSERT INTO inventory.warehouses (tenant_id, name, address, city, kind, manager_user_id)
        VALUES (p_tenant_id, TRIM(p_name), p_address, p_city, COALESCE(p_kind, 'store'), p_manager_user_id)
        RETURNING id INTO v_id;
        RETURN v_id;
    END IF;

    UPDATE inventory.warehouses w
    SET name = TRIM(p_name), address = p_address, city = p_city, kind = COALESCE(p_kind, w.kind),
        manager_user_id = p_manager_user_id, updated_at = now()
    WHERE w.id = v_id AND w.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory.warehouse.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_upsert_warehouse(uuid,uuid,character varying,character varying,character varying,character varying,uuid) IS 'Creates a warehouse, or changes the one named. Raises inventory.warehouse.not-found.';
