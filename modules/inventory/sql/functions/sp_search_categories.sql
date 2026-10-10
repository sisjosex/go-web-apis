CREATE OR REPLACE FUNCTION inventory.sp_search_categories(p_tenant_id uuid, p_search_term character varying, p_is_active boolean DEFAULT true, p_limit integer DEFAULT 50)
 RETURNS TABLE(id uuid, parent_id uuid, name character varying, slug character varying, description text, icon_url character varying, display_order integer, is_active boolean, product_count bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_search_pattern VARCHAR;
BEGIN
    v_search_pattern := '%' || LOWER(TRIM(p_search_term)) || '%';

    RETURN QUERY
    SELECT
        CAST(pc.id AS UUID),
        CAST(pc.parent_id AS UUID),
        CAST(pc.name AS VARCHAR),
        CAST(pc.slug AS VARCHAR),
        pc.description,
        CAST(pc.icon_url AS VARCHAR),
        pc.display_order,
        pc.is_active,
        CAST(COUNT(DISTINCT pcm.product_id) AS BIGINT)
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.tenant_id = p_tenant_id
        AND pc.deleted_at IS NULL
        AND (p_is_active IS NULL OR pc.is_active = p_is_active)
        AND (
            LOWER(pc.name) LIKE v_search_pattern
            OR LOWER(pc.slug) LIKE v_search_pattern
            OR LOWER(COALESCE(pc.description, '')) LIKE v_search_pattern
        )
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active
    ORDER BY pc.display_order ASC, pc.created_at DESC
    LIMIT p_limit;
END;
$function$;
