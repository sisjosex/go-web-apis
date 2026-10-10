CREATE OR REPLACE FUNCTION inventory.sp_assign_product_to_category(p_tenant_id uuid, p_product_id uuid, p_category_id uuid)
 RETURNS TABLE(id uuid, product_id uuid, category_id uuid, message character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_mapping_id UUID;
BEGIN
    -- Check product exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.products pr WHERE pr.id = p_product_id AND pr.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check category exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if already assigned
    IF EXISTS (SELECT 1 FROM inventory.product_category_mapping pcm WHERE pcm.product_id = p_product_id AND pcm.category_id = p_category_id) THEN
        RAISE EXCEPTION 'product.already-in-category' USING ERRCODE = 'P0001';
    END IF;

    -- Insert mapping
    INSERT INTO inventory.product_category_mapping (product_id, category_id)
    VALUES (p_product_id, p_category_id)
    RETURNING inventory.product_category_mapping.id INTO v_mapping_id;

    RETURN QUERY
    SELECT
        CAST(v_mapping_id AS UUID),
        CAST(p_product_id AS UUID),
        CAST(p_category_id AS UUID),
        CAST('Product assigned to category' AS VARCHAR);
END;
$function$;
