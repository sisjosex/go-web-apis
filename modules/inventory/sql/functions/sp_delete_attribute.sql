CREATE OR REPLACE FUNCTION inventory.sp_delete_attribute(p_tenant_id uuid, p_attribute_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_code TEXT;
BEGIN
    SELECT a.code INTO v_code FROM inventory.attributes a WHERE a.id = p_attribute_id AND a.tenant_id = p_tenant_id;
    IF v_code IS NULL THEN
        RAISE EXCEPTION 'inventory.attribute.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF EXISTS (SELECT 1 FROM inventory.products p WHERE p.tenant_id = p_tenant_id AND p.attributes ? v_code)
       OR EXISTS (SELECT 1 FROM inventory.product_variant_groups vg WHERE vg.attribute_id = p_attribute_id)
    THEN
        RAISE EXCEPTION 'inventory.attribute.in-use' USING ERRCODE = 'P0001';
    END IF;
    DELETE FROM inventory.attributes a WHERE a.id = p_attribute_id AND a.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_delete_attribute(uuid,uuid) IS 'Deletes an attribute no product uses. Raises inventory.attribute.not-found and inventory.attribute.in-use.';
