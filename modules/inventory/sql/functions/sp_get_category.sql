CREATE OR REPLACE FUNCTION inventory.sp_get_category(p_tenant_id uuid, p_category_id uuid)
 RETURNS TABLE(id uuid, parent_id uuid, name character varying, slug character varying, description text, icon_url character varying, display_order integer, is_active boolean, product_count bigint, created_at timestamp without time zone, updated_at timestamp without time zone, updated_by_name character varying, icon character varying, color character varying, pos_tab boolean)
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
        pc.updated_at,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        CAST(pc.icon AS VARCHAR),
        CAST(pc.color AS VARCHAR),
        pc.pos_tab
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    LEFT JOIN auth.users u ON u.id = pc.updated_by
    WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active, pc.created_at, pc.updated_at, pc.icon, pc.color, pc.pos_tab,
             u.first_name, u.last_name, u.email;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_get_category(uuid,uuid) IS 'One active category of the tenant with its product count and who last changed it (INV-020 D1).';
