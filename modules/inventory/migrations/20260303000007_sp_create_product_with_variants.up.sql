-- Create product with variants
CREATE OR REPLACE FUNCTION inventory.sp_create_product_with_variants(
    p_tenant_id UUID,
    p_sku VARCHAR,
    p_name VARCHAR,
    p_description TEXT,
    p_base_price DECIMAL,
    p_variants JSONB DEFAULT NULL,
    p_created_by UUID DEFAULT NULL
)
RETURNS TABLE(
    product_id UUID,
    sku VARCHAR,
    name VARCHAR,
    message TEXT
) LANGUAGE plpgsql AS $$
DECLARE
    v_product_id UUID;
    v_group JSONB;
    v_option JSONB;
    v_group_id UUID;
BEGIN
    -- Validate SKU unique per tenant
    IF EXISTS (SELECT 1 FROM inventory.products p WHERE p.sku = p_sku AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.sku.already-exists' USING ERRCODE = 'P0001';
    END IF;

    -- Create product
    INSERT INTO inventory.products (tenant_id, sku, name, description, base_price, has_variants)
    VALUES (p_tenant_id, p_sku, p_name, p_description, p_base_price, CASE WHEN p_variants IS NOT NULL THEN TRUE ELSE FALSE END)
    RETURNING products.id INTO v_product_id;

    -- Create initial stock
    INSERT INTO inventory.product_stock (product_id, current_quantity)
    VALUES (v_product_id, 0);

    -- Process variants if provided
    IF p_variants IS NOT NULL THEN
        FOR v_group IN SELECT jsonb_array_elements(p_variants -> 'groups')
        LOOP
            INSERT INTO inventory.product_variant_groups (
                product_id,
                group_type,
                is_required,
                max_selections
            )
            VALUES (
                v_product_id,
                v_group->>'group_type',
                (v_group->>'is_required')::BOOLEAN,
                (v_group->>'max_selections')::INT
            )
            RETURNING product_variant_groups.id INTO v_group_id;

            FOR v_option IN SELECT jsonb_array_elements(v_group -> 'options')
            LOOP
                INSERT INTO inventory.product_variant_options (
                    variant_group_id,
                    option_name,
                    price_modifier
                )
                VALUES (
                    v_group_id,
                    v_option->>'name',
                    (v_option->>'modifier')::DECIMAL
                );
            END LOOP;
        END LOOP;
    END IF;

    -- Return with explicit casting
    RETURN QUERY SELECT
        v_product_id,
        CAST(p_sku AS VARCHAR),
        CAST(p_name AS VARCHAR),
        CAST(CASE WHEN p_variants IS NOT NULL THEN 'Product created with variants' ELSE 'Product created' END AS TEXT);
END;
$$;
