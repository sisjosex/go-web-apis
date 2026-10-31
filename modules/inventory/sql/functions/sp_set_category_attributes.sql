CREATE OR REPLACE FUNCTION inventory.sp_set_category_attributes(p_tenant_id uuid, p_category_id uuid, p_attribute_ids uuid[])
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM inventory.product_categories pc
        WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF EXISTS (
        SELECT 1 FROM unnest(p_attribute_ids) x
        WHERE NOT EXISTS (SELECT 1 FROM inventory.attributes a WHERE a.id = x AND a.tenant_id = p_tenant_id)
    ) THEN
        RAISE EXCEPTION 'inventory.attribute.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM inventory.category_attributes ca WHERE ca.category_id = p_category_id AND ca.attribute_id <> ALL (p_attribute_ids);
    INSERT INTO inventory.category_attributes (category_id, attribute_id)
    SELECT p_category_id, x FROM unnest(p_attribute_ids) x
    ON CONFLICT DO NOTHING;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_category_attributes(uuid,uuid,uuid[]) IS 'The attributes a category brings of its own (INV-024 D3).';
