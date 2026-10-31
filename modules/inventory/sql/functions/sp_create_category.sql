CREATE OR REPLACE FUNCTION inventory.sp_create_category(p_tenant_id uuid, p_name character varying, p_slug character varying, p_parent_id uuid DEFAULT NULL::uuid, p_description text DEFAULT NULL::text, p_icon_url character varying DEFAULT NULL::character varying, p_display_order integer DEFAULT 0, p_created_by uuid DEFAULT NULL::uuid, p_icon character varying DEFAULT NULL::character varying, p_color character varying DEFAULT NULL::character varying, p_pos_tab boolean DEFAULT NULL::boolean)
 RETURNS TABLE(id uuid, parent_id uuid, name character varying, slug character varying, description text, icon_url character varying, display_order integer, is_active boolean, product_count bigint, created_at timestamp without time zone, updated_at timestamp without time zone, icon character varying, color character varying, pos_tab boolean)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_category_id UUID;
    v_name VARCHAR := TRIM(p_name);
    v_slug VARCHAR := TRIM(p_slug);
BEGIN
    -- Validate inputs
    IF v_name = '' THEN
        RAISE EXCEPTION 'category.name-required' USING ERRCODE = 'P0001';
    END IF;

    IF v_slug = '' THEN
        RAISE EXCEPTION 'category.slug-required' USING ERRCODE = 'P0001';
    END IF;

    -- Check slug uniqueness per tenant
    IF EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.tenant_id = p_tenant_id AND pc.slug = v_slug AND pc.deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.slug-already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Validate parent exists and belongs to tenant
    IF p_parent_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM inventory.product_categories pc WHERE pc.id = p_parent_id AND pc.tenant_id = p_tenant_id AND pc.deleted_at IS NULL) THEN
            RAISE EXCEPTION 'category.parent-not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Check circular hierarchy
    IF p_parent_id IS NOT NULL THEN
        IF NOT inventory.sp_check_circular_hierarchy(p_tenant_id, NULL, p_parent_id) THEN
            RAISE EXCEPTION 'category.circular-hierarchy' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Insert category
    INSERT INTO inventory.product_categories (tenant_id, parent_id, name, slug, description, icon_url, display_order, created_by, updated_by, icon, color, pos_tab)
    VALUES (p_tenant_id, p_parent_id, v_name, v_slug, p_description, p_icon_url, p_display_order, p_created_by, p_created_by, p_icon, p_color, COALESCE(p_pos_tab, TRUE))
    RETURNING inventory.product_categories.id INTO v_category_id;

    -- Return created category (same fields as sp_get_category for consistency)
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
        CAST(pc.icon AS VARCHAR),
        CAST(pc.color AS VARCHAR),
        pc.pos_tab
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.id = v_category_id
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url,
             pc.display_order, pc.is_active, pc.created_at, pc.updated_at, pc.icon, pc.color, pc.pos_tab;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_create_category(uuid,character varying,character varying,uuid,text,character varying,integer,uuid,character varying,character varying,boolean) IS 'Creates a category of the tenant, recording who did it (INV-020 D1), and answers it as sp_get_category does; refuses an empty name or slug, a slug in use and an unknown parent.';
