-- INV-017 D1 — the read-only lookups the CSV import descriptors ran as inline
-- SQL from the service layer, moved behind one repository. Every one of them
-- takes p_tenant_id and filters on it in every statement: the variant option
-- lookup used to match on the product id alone.
--
-- They only read. Deciding what a miss means — sku-unknown, axes-incomplete,
-- combination-unknown, a category to be created — stays with the descriptor,
-- which is the one that knows whether the run is a dry run.

DROP FUNCTION IF EXISTS inventory.sp_import_resolve_sku_by_axes(
    UUID, VARCHAR, TEXT[], TEXT[]
);
CREATE FUNCTION inventory.sp_import_resolve_sku_by_axes(
    p_tenant_id UUID,
    p_sku       VARCHAR,
    p_axes      TEXT[],
    p_options   TEXT[]
)
RETURNS TABLE(
    product_id     UUID,
    axis_count     INT,
    matched_sku_id UUID,
    matched_sku    VARCHAR,
    default_sku_id UUID,
    default_sku    VARCHAR
) LANGUAGE plpgsql STABLE AS $$
DECLARE
    v_product_id UUID;
    v_wanted     TEXT[];
BEGIN
    -- The cell names either the product's own SKU or one of its combinations.
    SELECT t.product_id INTO v_product_id
    FROM (
        SELECT p.id AS product_id
        FROM inventory.products p
        WHERE p.tenant_id    = p_tenant_id
          AND LOWER(p.sku)   = LOWER(TRIM(p_sku))
        UNION
        SELECT s.product_id
        FROM inventory.product_skus s
        WHERE s.tenant_id    = p_tenant_id
          AND LOWER(s.sku)   = LOWER(TRIM(p_sku))
    ) t
    LIMIT 1;

    -- One row always, NULLs when the product is unknown, so the caller tells the
    -- three misses apart from the facts rather than from an empty result.
    IF v_product_id IS NULL
    THEN
        RETURN QUERY SELECT
            CAST(NULL AS UUID),
            CAST(0 AS INT),
            CAST(NULL AS UUID),
            CAST(NULL AS VARCHAR),
            CAST(NULL AS UUID),
            CAST(NULL AS VARCHAR);
        RETURN;
    END IF;

    v_wanted := ARRAY(
        SELECT LOWER(TRIM(w.axis)) || '=' || LOWER(TRIM(w.option))
        FROM unnest(COALESCE(p_axes, ARRAY[]::TEXT[]),
                    COALESCE(p_options, ARRAY[]::TEXT[])) AS w(axis, option)
        ORDER BY 1
    );

    RETURN QUERY SELECT
        v_product_id,
        CAST((
            SELECT COUNT(*)
            FROM inventory.product_variant_groups vg
            JOIN inventory.products p ON p.id = vg.product_id
            WHERE vg.product_id = v_product_id
              AND p.tenant_id   = p_tenant_id
              AND vg.affects_inventory
        ) AS INT),
        m.id,
        CAST(m.sku AS VARCHAR),
        d.id,
        CAST(d.sku AS VARCHAR)
    FROM (SELECT 1) AS one
    -- Exactly the row's option set, never a subset (INV-016 D6).
    LEFT JOIN LATERAL (
        SELECT s.id, s.sku
        FROM inventory.product_skus s
        WHERE s.product_id = v_product_id
          AND s.tenant_id  = p_tenant_id
          AND NOT s.is_default
          AND ARRAY(
                  SELECT LOWER(g.group_type) || '=' || LOWER(vo.option_name)
                  FROM inventory.product_sku_options so
                  JOIN inventory.product_variant_groups g
                    ON g.id = so.variant_group_id
                  JOIN inventory.product_variant_options vo
                    ON vo.id = so.option_id
                  WHERE so.sku_id = s.id
                  ORDER BY 1
              ) = v_wanted
        LIMIT 1
    ) m ON TRUE
    LEFT JOIN LATERAL (
        SELECT s.id, s.sku
        FROM inventory.product_skus s
        WHERE s.product_id = v_product_id
          AND s.tenant_id  = p_tenant_id
          AND s.is_default
        LIMIT 1
    ) d ON TRUE;
END;
$$;
COMMENT ON FUNCTION inventory.sp_import_resolve_sku_by_axes IS
    'Resolves the product a CSV cell names (by product or combination SKU) and '
    'the combination whose option set equals p_axes/p_options exactly. Always '
    'one row: product_id NULL when the SKU is unknown, axis_count the '
    'product''s inventory axes, matched_* the exact combination, default_* the '
    'default bucket. Read-only, tenant-scoped (INV-017 D1).';

DROP FUNCTION IF EXISTS inventory.sp_import_find_category(UUID, VARCHAR, UUID);
CREATE FUNCTION inventory.sp_import_find_category(
    p_tenant_id UUID,
    p_name      VARCHAR,
    p_parent_id UUID DEFAULT NULL
)
RETURNS TABLE(
    category_id UUID
) LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN QUERY
    SELECT c.id
    FROM inventory.product_categories c
    WHERE c.tenant_id       = p_tenant_id
      AND LOWER(c.name)     = LOWER(TRIM(p_name))
      AND c.deleted_at IS NULL
      AND (p_parent_id IS NULL OR c.parent_id = p_parent_id)
    ORDER BY c.created_at
    LIMIT 1;
END;
$$;
COMMENT ON FUNCTION inventory.sp_import_find_category IS
    'The oldest live category of the tenant with this name, case-insensitive; '
    'p_parent_id narrows it to the children of one category. No row when there '
    'is none. Read-only (INV-017 D1).';

DROP FUNCTION IF EXISTS inventory.sp_import_find_variant_option(
    UUID, UUID, VARCHAR, VARCHAR
);
CREATE FUNCTION inventory.sp_import_find_variant_option(
    p_tenant_id  UUID,
    p_product_id UUID,
    p_axis       VARCHAR,
    p_option     VARCHAR
)
RETURNS TABLE(
    option_id UUID
) LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN QUERY
    SELECT vo.id
    FROM inventory.product_variant_options vo
    JOIN inventory.product_variant_groups vg ON vg.id = vo.variant_group_id
    JOIN inventory.products p ON p.id = vg.product_id
    WHERE vg.product_id          = p_product_id
      AND p.tenant_id            = p_tenant_id
      AND vg.affects_inventory
      AND LOWER(vg.group_type)   = LOWER(TRIM(p_axis))
      AND LOWER(vo.option_name)  = LOWER(TRIM(p_option))
    LIMIT 1;
END;
$$;
COMMENT ON FUNCTION inventory.sp_import_find_variant_option IS
    'The option of one inventory axis of a tenant''s product, matched '
    'case-insensitively, that an imported picture hangs off (INV-016 D7). No '
    'row when the product is not the tenant''s or has no such option. '
    'Read-only (INV-017 D1).';

DROP FUNCTION IF EXISTS inventory.sp_import_sku_exists(UUID, VARCHAR);
CREATE FUNCTION inventory.sp_import_sku_exists(
    p_tenant_id UUID,
    p_sku       VARCHAR
)
RETURNS TABLE(
    product_sku_taken BOOLEAN,
    variant_sku_taken BOOLEAN
) LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN QUERY SELECT
        EXISTS (
            SELECT 1
            FROM inventory.products p
            WHERE p.tenant_id  = p_tenant_id
              AND LOWER(p.sku) = LOWER(TRIM(p_sku))
        ),
        EXISTS (
            SELECT 1
            FROM inventory.product_skus s
            WHERE s.tenant_id  = p_tenant_id
              AND LOWER(s.sku) = LOWER(TRIM(p_sku))
        );
END;
$$;
COMMENT ON FUNCTION inventory.sp_import_sku_exists IS
    'Whether the tenant already has a product (product_sku_taken) or a '
    'combination (variant_sku_taken) with this code, so the dry run previews '
    'the duplicate the create SPs would refuse. Read-only (INV-017 D1).';

DROP FUNCTION IF EXISTS inventory.sp_import_free_category_slug(UUID, VARCHAR, INT);
CREATE FUNCTION inventory.sp_import_free_category_slug(
    p_tenant_id UUID,
    p_base      VARCHAR,
    p_limit     INT DEFAULT 20
)
RETURNS TABLE(
    slug VARCHAR
) LANGUAGE plpgsql STABLE AS $$
DECLARE
    v_candidate VARCHAR := p_base;
    v_suffix    INT;
BEGIN
    -- product_categories.slug is UNIQUE per tenant, deleted rows included, so
    -- two categories named the same under different parents need base, base-2…
    FOR v_suffix IN 1..p_limit
    LOOP
        IF v_suffix > 1
        THEN
            v_candidate := CAST(p_base || '-' || v_suffix AS VARCHAR);
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM inventory.product_categories c
            WHERE c.tenant_id = p_tenant_id
              AND c.slug      = v_candidate
        )
        THEN
            RETURN QUERY SELECT v_candidate;
            RETURN;
        END IF;
    END LOOP;

    -- Every candidate taken: the base comes back and sp_create_category refuses
    -- it, which is the row error the caller already reports.
    RETURN QUERY SELECT CAST(p_base AS VARCHAR);
END;
$$;
COMMENT ON FUNCTION inventory.sp_import_free_category_slug IS
    'The first of p_base, p_base-2 … p_base-<p_limit> the tenant is not using as '
    'a category slug; p_base itself when every one is taken. Read-only '
    '(INV-017 D1).';
