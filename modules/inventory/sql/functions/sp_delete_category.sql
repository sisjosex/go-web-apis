CREATE OR REPLACE FUNCTION inventory.sp_delete_category(p_tenant_id uuid, p_category_id uuid)
 RETURNS TABLE(id uuid, name character varying, deleted boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check category exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if category has products
    IF EXISTS (SELECT 1 FROM inventory.product_category_mapping pcm WHERE pcm.category_id = p_category_id) THEN
        RAISE EXCEPTION 'category.has-products' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete
    UPDATE inventory.product_categories pc
    SET deleted_at = NOW()
    WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id;

    -- Return deleted category
    RETURN QUERY
    SELECT
        CAST(pc.id AS UUID),
        CAST(pc.name AS VARCHAR),
        TRUE AS deleted
    FROM inventory.product_categories pc
    WHERE pc.id = p_category_id;
END;
$function$;
