CREATE OR REPLACE FUNCTION inventory.sp_remove_product_from_category(p_tenant_id uuid, p_product_id uuid, p_category_id uuid)
 RETURNS TABLE(product_id uuid, category_id uuid, removed boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
    DELETE FROM inventory.product_category_mapping pcm
    WHERE pcm.product_id = p_product_id AND pcm.category_id = p_category_id;

    RETURN QUERY
    SELECT
        CAST(p_product_id AS UUID),
        CAST(p_category_id AS UUID),
        TRUE AS removed;
END;
$function$;
