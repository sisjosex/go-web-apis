CREATE OR REPLACE FUNCTION inventory.sp_get_products_by_category(p_tenant_id uuid, p_category_id uuid, p_limit integer DEFAULT 20, p_offset integer DEFAULT 0)
 RETURNS TABLE(id uuid, name character varying, sku character varying, description text, base_price numeric, status character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check category exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        CAST(pr.id AS UUID),
        CAST(pr.name AS VARCHAR),
        CAST(pr.sku AS VARCHAR),
        pr.description,
        pr.base_price,
        CAST(pr.status AS VARCHAR)
    FROM inventory.products pr
    INNER JOIN inventory.product_category_mapping pcm ON pr.id = pcm.product_id
    WHERE pcm.category_id = p_category_id AND pr.tenant_id = p_tenant_id
    ORDER BY pr.created_at DESC
    LIMIT p_limit OFFSET p_offset;
END;
$function$;
