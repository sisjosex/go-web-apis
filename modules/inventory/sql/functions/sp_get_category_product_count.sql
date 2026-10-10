CREATE OR REPLACE FUNCTION inventory.sp_get_category_product_count(p_tenant_id uuid, p_category_id uuid)
 RETURNS TABLE(category_id uuid, product_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        CAST(p_category_id AS UUID),
        CAST(COUNT(pcm.product_id) AS BIGINT)
    FROM inventory.product_category_mapping pcm
    INNER JOIN inventory.products pr ON pr.id = pcm.product_id
    WHERE pcm.category_id = p_category_id AND pr.tenant_id = p_tenant_id;
END;
$function$;
