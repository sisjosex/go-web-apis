CREATE OR REPLACE FUNCTION inventory.fn_default_warehouse(p_tenant_id uuid)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
BEGIN
    SELECT w.id INTO v_id FROM inventory.warehouses w WHERE w.tenant_id = p_tenant_id AND w.is_default;
    IF v_id IS NOT NULL THEN
        RETURN v_id;
    END IF;

    INSERT INTO inventory.warehouses (tenant_id, name, is_default)
    VALUES (p_tenant_id, 'Principal', TRUE)
    ON CONFLICT (tenant_id) WHERE is_default DO NOTHING
    RETURNING id INTO v_id;

    -- Lost a race with another first movement: its Principal is this one.
    IF v_id IS NULL THEN
        SELECT w.id INTO v_id FROM inventory.warehouses w WHERE w.tenant_id = p_tenant_id AND w.is_default;
    END IF;
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_default_warehouse(uuid) IS 'A business''s default warehouse, creating Principal the first time it has none (INV-025 D1).';
