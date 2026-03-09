-- Product Categories Stored Procedures

-- ===== DROP EXISTING FUNCTIONS =====
-- Drop old versions to handle signature changes
DROP FUNCTION IF EXISTS inventory.sp_create_category(VARCHAR, VARCHAR, UUID, TEXT, VARCHAR, INTEGER) CASCADE;
DROP FUNCTION IF EXISTS inventory.sp_update_category(UUID, VARCHAR, VARCHAR, UUID, TEXT, VARCHAR, INTEGER, BOOLEAN) CASCADE;

-- ===== HELPERS =====

-- sp_check_circular_hierarchy: Validates no circular references
CREATE OR REPLACE FUNCTION inventory.sp_check_circular_hierarchy(
    p_category_id UUID,
    p_parent_id UUID
)
RETURNS BOOLEAN LANGUAGE plpgsql AS $$
DECLARE
    v_current_id UUID := p_parent_id;
    v_iterations INT := 0;
BEGIN
    -- Prevent infinite loops
    WHILE v_current_id IS NOT NULL AND v_iterations < 100 LOOP
        IF v_current_id = p_category_id THEN
            RETURN FALSE;  -- Circular reference detected
        END IF;
        SELECT parent_id INTO v_current_id
        FROM inventory.product_categories
        WHERE id = v_current_id AND deleted_at IS NULL;
        v_iterations := v_iterations + 1;
    END LOOP;
    RETURN TRUE;  -- No circular reference
END;
$$;

-- ===== CRUD OPERATIONS =====

-- sp_create_category: Create new category
CREATE OR REPLACE FUNCTION inventory.sp_create_category(
    p_name VARCHAR,
    p_slug VARCHAR,
    p_parent_id UUID DEFAULT NULL,
    p_description TEXT DEFAULT NULL,
    p_icon_url VARCHAR DEFAULT NULL,
    p_display_order INTEGER DEFAULT 0
)
RETURNS TABLE (
    id UUID,
    parent_id UUID,
    name VARCHAR,
    slug VARCHAR,
    description TEXT,
    icon_url VARCHAR,
    display_order INTEGER,
    is_active BOOLEAN,
    product_count BIGINT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_category_id UUID;
BEGIN
    -- Validate inputs
    IF TRIM(p_name) = '' THEN
        RAISE EXCEPTION 'category.name-required' USING ERRCODE = 'P0001';
    END IF;
    
    IF TRIM(p_slug) = '' THEN
        RAISE EXCEPTION 'category.slug-required' USING ERRCODE = 'P0001';
    END IF;

    -- Check slug uniqueness
    IF EXISTS (SELECT 1 FROM inventory.product_categories WHERE slug = p_slug AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.slug-already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Validate parent exists
    IF p_parent_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_parent_id AND deleted_at IS NULL) THEN
            RAISE EXCEPTION 'category.parent-not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Check circular hierarchy
    IF p_parent_id IS NOT NULL THEN
        IF NOT inventory.sp_check_circular_hierarchy(NULL, p_parent_id) THEN
            RAISE EXCEPTION 'category.circular-hierarchy' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Insert category
    INSERT INTO inventory.product_categories (parent_id, name, slug, description, icon_url, display_order)
    VALUES (p_parent_id, TRIM(p_name), TRIM(p_slug), p_description, p_icon_url, p_display_order)
    RETURNING product_categories.id INTO v_category_id;

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
        pc.updated_at
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.id = v_category_id
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url, 
             pc.display_order, pc.is_active, pc.created_at, pc.updated_at;
END;
$$;
END;
$$;

-- sp_update_category: Update category
CREATE OR REPLACE FUNCTION inventory.sp_update_category(
    p_category_id UUID,
    p_name VARCHAR DEFAULT NULL,
    p_slug VARCHAR DEFAULT NULL,
    p_parent_id UUID DEFAULT NULL,
    p_description TEXT DEFAULT NULL,
    p_icon_url VARCHAR DEFAULT NULL,
    p_display_order INTEGER DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    parent_id UUID,
    name VARCHAR,
    slug VARCHAR,
    description TEXT,
    icon_url VARCHAR,
    display_order INTEGER,
    is_active BOOLEAN,
    product_count BIGINT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    -- Check category exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_category_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check slug uniqueness (if updating slug)
    IF p_slug IS NOT NULL AND TRIM(p_slug) != '' THEN
        IF EXISTS (
            SELECT 1 FROM inventory.product_categories 
            WHERE slug = TRIM(p_slug) AND id != p_category_id AND deleted_at IS NULL
        ) THEN
            RAISE EXCEPTION 'category.slug-already-exists' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Validate parent (if updating parent)
    IF p_parent_id IS NOT NULL THEN
        IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_parent_id AND deleted_at IS NULL) THEN
            RAISE EXCEPTION 'category.parent-not-found' USING ERRCODE = 'P0001';
        END IF;
        -- Check circular hierarchy
        IF NOT inventory.sp_check_circular_hierarchy(p_category_id, p_parent_id) THEN
            RAISE EXCEPTION 'category.circular-hierarchy' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- Update category
    UPDATE inventory.product_categories SET
        parent_id = COALESCE(p_parent_id, parent_id),
        name = COALESCE(NULLIF(TRIM(p_name), ''), name),
        slug = COALESCE(NULLIF(TRIM(p_slug), ''), slug),
        description = COALESCE(p_description, description),
        icon_url = COALESCE(p_icon_url, icon_url),
        display_order = COALESCE(p_display_order, display_order),
        is_active = COALESCE(p_is_active, is_active),
        updated_at = NOW()
    WHERE id = p_category_id;

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
$$;

-- sp_delete_category: Soft delete category
CREATE OR REPLACE FUNCTION inventory.sp_delete_category(p_category_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    deleted BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    -- Check category exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_category_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if category has products
    IF EXISTS (SELECT 1 FROM inventory.product_category_mapping WHERE category_id = p_category_id) THEN
        RAISE EXCEPTION 'category.has-products' USING ERRCODE = 'P0001';
    END IF;

    -- Soft delete
    UPDATE inventory.product_categories
    SET deleted_at = NOW()
    WHERE id = p_category_id;

    -- Return deleted category
    RETURN QUERY
    SELECT 
        CAST(pc.id AS UUID),
        CAST(pc.name AS VARCHAR),
        TRUE AS deleted
    FROM inventory.product_categories pc
    WHERE pc.id = p_category_id;
END;
$$;

-- sp_get_category: Get single category
CREATE OR REPLACE FUNCTION inventory.sp_get_category(p_category_id UUID)
RETURNS TABLE (
    id UUID,
    parent_id UUID,
    name VARCHAR,
    slug VARCHAR,
    description TEXT,
    icon_url VARCHAR,
    display_order INTEGER,
    is_active BOOLEAN,
    product_count BIGINT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
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
        pc.updated_at
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.id = p_category_id AND pc.deleted_at IS NULL
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url, 
             pc.display_order, pc.is_active, pc.created_at, pc.updated_at;
END;
$$;

-- ===== LIST & SEARCH =====

-- sp_list_categories: List categories by parent (or all if NULL)
CREATE OR REPLACE FUNCTION inventory.sp_list_categories(
    p_parent_id UUID DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT TRUE,
    p_order_by VARCHAR DEFAULT 'display_order',
    p_limit INTEGER DEFAULT 1000,
    p_offset INTEGER DEFAULT 0
)
RETURNS TABLE (
    id UUID,
    parent_id UUID,
    name VARCHAR,
    slug VARCHAR,
    description TEXT,
    icon_url VARCHAR,
    display_order INTEGER,
    is_active BOOLEAN,
    product_count BIGINT,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
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
        pc.created_at
    FROM inventory.product_categories pc
    LEFT JOIN inventory.product_category_mapping pcm ON pc.id = pcm.category_id
    WHERE pc.deleted_at IS NULL
        AND (p_parent_id IS NULL OR pc.parent_id = p_parent_id)
        AND (p_is_active IS NULL OR pc.is_active = p_is_active)
    GROUP BY pc.id, pc.parent_id, pc.name, pc.slug, pc.description, pc.icon_url, 
             pc.display_order, pc.is_active, pc.created_at
    ORDER BY 
        CASE WHEN p_order_by = 'name' THEN pc.name END ASC,
        CASE WHEN p_order_by = 'display_order' THEN pc.display_order END ASC,
        CASE WHEN p_order_by = 'created_at' THEN pc.created_at END DESC
    LIMIT p_limit OFFSET p_offset;
END;
$$;

-- sp_search_categories: Search categories by name/slug/description
CREATE OR REPLACE FUNCTION inventory.sp_search_categories(
    p_search_term VARCHAR,
    p_is_active BOOLEAN DEFAULT TRUE,
    p_limit INTEGER DEFAULT 50
)
RETURNS TABLE (
    id UUID,
    parent_id UUID,
    name VARCHAR,
    slug VARCHAR,
    description TEXT,
    icon_url VARCHAR,
    display_order INTEGER,
    is_active BOOLEAN,
    product_count BIGINT
) LANGUAGE plpgsql AS $$
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
    WHERE pc.deleted_at IS NULL
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
$$;

-- ===== PRODUCT-CATEGORY MAPPING =====

-- sp_assign_product_to_category: Assign product to category
CREATE OR REPLACE FUNCTION inventory.sp_assign_product_to_category(
    p_product_id UUID,
    p_category_id UUID
)
RETURNS TABLE (
    id UUID,
    product_id UUID,
    category_id UUID,
    message VARCHAR
) LANGUAGE plpgsql AS $$
DECLARE
    v_mapping_id UUID;
BEGIN
    -- Check product exists
    IF NOT EXISTS (SELECT 1 FROM inventory.products WHERE id = p_product_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check category exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_category_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Check if already assigned
    IF EXISTS (SELECT 1 FROM inventory.product_category_mapping WHERE product_id = p_product_id AND category_id = p_category_id) THEN
        RAISE EXCEPTION 'product.already-in-category' USING ERRCODE = 'P0001';
    END IF;

    -- Insert mapping
    INSERT INTO inventory.product_category_mapping (product_id, category_id)
    VALUES (p_product_id, p_category_id)
    RETURNING product_category_mapping.id INTO v_mapping_id;

    RETURN QUERY
    SELECT 
        CAST(v_mapping_id AS UUID),
        CAST(p_product_id AS UUID),
        CAST(p_category_id AS UUID),
        CAST('Product assigned to category' AS VARCHAR);
END;
$$;

-- sp_remove_product_from_category: Remove product from category
CREATE OR REPLACE FUNCTION inventory.sp_remove_product_from_category(
    p_product_id UUID,
    p_category_id UUID
)
RETURNS TABLE (
    product_id UUID,
    category_id UUID,
    removed BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM inventory.product_category_mapping
    WHERE product_id = p_product_id AND category_id = p_category_id;

    RETURN QUERY
    SELECT 
        CAST(p_product_id AS UUID),
        CAST(p_category_id AS UUID),
        TRUE AS removed;
END;
$$;

-- sp_get_products_by_category: Get products in category (paginated)
CREATE OR REPLACE FUNCTION inventory.sp_get_products_by_category(
    p_category_id UUID,
    p_limit INTEGER DEFAULT 20,
    p_offset INTEGER DEFAULT 0
)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    sku VARCHAR,
    description TEXT,
    price DECIMAL,
    cost_price DECIMAL,
    quantity_on_hand BIGINT,
    reorder_level BIGINT,
    status VARCHAR,
    image_url VARCHAR
) LANGUAGE plpgsql AS $$
BEGIN
    -- Check category exists
    IF NOT EXISTS (SELECT 1 FROM inventory.product_categories WHERE id = p_category_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'category.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT 
        CAST(pr.id AS UUID),
        CAST(pr.name AS VARCHAR),
        CAST(pr.sku AS VARCHAR),
        pr.description,
        pr.price,
        pr.cost_price,
        pr.quantity_on_hand,
        pr.reorder_level,
        CAST(pr.status AS VARCHAR),
        CAST(pr.image_url AS VARCHAR)
    FROM inventory.products pr
    INNER JOIN inventory.product_category_mapping pcm ON pr.id = pcm.product_id
    WHERE pcm.category_id = p_category_id
    ORDER BY pr.created_at DESC
    LIMIT p_limit OFFSET p_offset;
END;
$$;

-- sp_get_category_product_count: Get count of products in category
CREATE OR REPLACE FUNCTION inventory.sp_get_category_product_count(p_category_id UUID)
RETURNS TABLE (
    category_id UUID,
    product_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        CAST(p_category_id AS UUID),
        CAST(COUNT(pcm.product_id) AS BIGINT)
    FROM inventory.product_category_mapping pcm
    WHERE pcm.category_id = p_category_id;
END;
$$;
