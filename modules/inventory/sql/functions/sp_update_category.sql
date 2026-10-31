CREATE OR REPLACE FUNCTION inventory.sp_update_category(p_tenant_id uuid, p_category_id uuid, p_name character varying DEFAULT NULL::character varying, p_slug character varying DEFAULT NULL::character varying, p_parent_id uuid DEFAULT NULL::uuid, p_description text DEFAULT NULL::text, p_icon_url character varying DEFAULT NULL::character varying, p_display_order integer DEFAULT NULL::integer, p_is_active boolean DEFAULT NULL::boolean, p_updated_by uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, parent_id uuid, name character varying, slug character varying, description text, icon_url character varying, display_order integer, is_active boolean, product_count bigint, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_new_slug VARCHAR;
BEGIN
    -- Check category exists and belongs to tenant
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check slug uniqueness (if updating slug)
    IF p_slug IS NOT NULL AND TRIM(p_slug) != '' THEN
        v_new_slug := TRIM(p_slug);
        IF EXISTS (
            SELECT 1 FROM inventory.product_categories pc
            WHERE pc.tenant_id = p_tenant_id AND pc.slug = v_new_slug AND pc.id != p_category_id AND pc.deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'category.slug-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Validate parent (if updating parent)
    IF p_parent_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_parent_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
            RAISE EXCEPTION 'category.parent-not-found' USING ERRCODE = 'P0001';
        END IF;
        -- Check circular hierarchy
        IF NOT inventory.sp_check_circular_hierarchy(p_tenant_id, p_category_id, p_parent_id) THEN
            RAISE EXCEPTION 'category.circular-hierarchy' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Update category
    UPDATE inventory.product_categories pc SET
        parent_id = COALESCE(p_parent_id, pc.parent_id),
        name = COALESCE(NULLIF(TRIM(p_name), ''), pc.name),
        slug = COALESCE(NULLIF(TRIM(p_slug), ''), pc.slug),
        description = COALESCE(p_description, pc.description),
        icon_url = COALESCE(p_icon_url, pc.icon_url),
        display_order = COALESCE(p_display_order, pc.display_order),
        is_active = COALESCE(p_is_active, pc.is_active),
        updated_by = p_updated_by,
        updated_at = NOW()
    WHERE pc.id = p_category_id AND pc.tenant_id = p_tenant_id;

    -- Return updated category (same fields as sp_get_category for consistency)
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
        pc.updated_at
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.id = p_category_id
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active, pc.created_at, pc.updated_at;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_update_category(uuid,uuid,character varying,character varying,uuid,text,character varying,integer,boolean,uuid) IS 'Changes the given fields of a category of the tenant, a NULL leaves one as it is, recording who did it (INV-020 D1), and answers it as sp_get_category does.';
