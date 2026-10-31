CREATE OR REPLACE FUNCTION inventory.sp_remove_product_from_category(p_tenant_id uuid, p_product_id uuid, p_category_id uuid)
 RETURNS TABLE(product_id uuid, category_id uuid, removed boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    DELETE FROM inventory.product_category_mapping pcm
    USING inventory.products p
    WHERE pcm.product_id = p_product_id
      AND pcm.category_id = p_category_id
      AND p.id = pcm.product_id
      AND p.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'inventory.category-mapping.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(p_product_id AS UUID),
        CAST(p_category_id AS UUID),
        TRUE AS removed;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_remove_product_from_category(uuid,uuid,uuid) IS 'Takes a product of this tenant out of a category; a mapping it cannot see, or that does not exist, raises inventory.category-mapping.not-found (INV-018).';
