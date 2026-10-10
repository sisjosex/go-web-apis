CREATE OR REPLACE FUNCTION inventory.sp_import_resolve_sku_by_axes(p_tenant_id uuid, p_sku character varying, p_axes text[], p_options text[])
 RETURNS TABLE(product_id uuid, axis_count integer, matched_sku_id uuid, matched_sku character varying, default_sku_id uuid, default_sku character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.sp_import_resolve_sku_by_axes(uuid,character varying,text[],text[]) IS 'Resolves the product a CSV cell names (by product or combination SKU) and the combination whose option set equals p_axes/p_options exactly. Always one row: product_id NULL when the SKU is unknown, axis_count the product''s inventory axes, matched_* the exact combination, default_* the default bucket. Read-only, tenant-scoped (INV-017 D1).';
