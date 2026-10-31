CREATE OR REPLACE FUNCTION inventory.sp_list_categories(p_tenant_id uuid, p_parent_id uuid DEFAULT NULL::uuid, p_is_active boolean DEFAULT true, p_order_by character varying DEFAULT 'display_order'::character varying, p_limit integer DEFAULT 1000, p_offset integer DEFAULT 0)
 RETURNS TABLE(id uuid, parent_id uuid, name character varying, slug character varying, description text, icon_url character varying, display_order integer, is_active boolean, product_count bigint, created_at timestamp without time zone, icon character varying, color character varying, pos_tab boolean)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
        CAST(COUNT(DISTINCT pcm.product_id) AS BIGINT),
        pc.created_at,
        CAST(pc.icon AS VARCHAR),
        CAST(pc.color AS VARCHAR),
        pc.pos_tab
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.tenant_id = p_tenant_id
        AND pc.deleted_at IS NULL
        AND (p_parent_id IS NULL OR pc.parent_id = p_parent_id)
        AND (p_is_active IS NULL OR pc.is_active = p_is_active)
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active, pc.created_at, pc.icon, pc.color, pc.pos_tab
    ORDER BY
        CASE WHEN p_order_by = 'name' THEN pc.name END ASC,
        CASE WHEN p_order_by = 'display_order' THEN pc.display_order END ASC,
        CASE WHEN p_order_by = 'created_at' THEN pc.created_at END DESC
    LIMIT p_limit OFFSET p_offset;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_list_categories(uuid,uuid,boolean,character varying,integer,integer) IS 'A page of a tenant''s categories with their product count and how they look in the POS (INV-024).';
