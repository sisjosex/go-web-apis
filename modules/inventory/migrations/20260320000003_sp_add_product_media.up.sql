CREATE OR REPLACE FUNCTION inventory.sp_add_product_media(
    p_tenant_id       UUID,
    p_product_id      UUID,
    p_variant_option_id UUID,
    p_media_type      VARCHAR,
    p_url             TEXT,
    p_alt_text        VARCHAR,
    p_is_primary      BOOLEAN,
    p_sort_order      INT
)
RETURNS TABLE(media_id UUID, message VARCHAR) LANGUAGE plpgsql AS $$
DECLARE
    v_media_id UUID;
BEGIN
    -- Verify product belongs to tenant
    IF NOT EXISTS (
        SELECT 1 FROM inventory.products
        WHERE id = p_product_id AND tenant_id = p_tenant_id AND status = 'active'
    ) THEN
        RAISE EXCEPTION 'I0001: Product not found';
    END IF;

    -- Verify variant option belongs to the product (when provided)
    IF p_variant_option_id IS NOT NULL THEN
        IF NOT EXISTS (
            SELECT 1
            FROM inventory.product_variant_options vo
            JOIN inventory.product_variant_groups vg ON vg.id = vo.variant_group_id
            WHERE vo.id = p_variant_option_id AND vg.product_id = p_product_id
        ) THEN
            RAISE EXCEPTION 'I0003: Variant option does not belong to this product';
        END IF;
    END IF;

    -- If is_primary, unset other primaries in the same scope
    IF p_is_primary THEN
        IF p_variant_option_id IS NULL THEN
            UPDATE inventory.product_media
            SET is_primary = false
            WHERE product_id = p_product_id
              AND variant_option_id IS NULL
              AND media_type = p_media_type;
        ELSE
            UPDATE inventory.product_media
            SET is_primary = false
            WHERE variant_option_id = p_variant_option_id
              AND media_type = p_media_type;
        END IF;
    END IF;

    INSERT INTO inventory.product_media
        (tenant_id, product_id, variant_option_id, media_type, url, alt_text, is_primary, sort_order)
    VALUES
        (p_tenant_id, p_product_id, p_variant_option_id, p_media_type, p_url, p_alt_text, p_is_primary, p_sort_order)
    RETURNING id INTO v_media_id;

    RETURN QUERY SELECT v_media_id, 'Media added successfully'::VARCHAR;
END;
$$;
