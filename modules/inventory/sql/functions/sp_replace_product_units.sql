CREATE OR REPLACE FUNCTION inventory.sp_replace_product_units(p_tenant_id uuid, p_product_id uuid, p_units jsonb)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_row    JSONB;
    v_id     UUID;
    v_kept   UUID[] := ARRAY[]::UUID[];
    v_gone   UUID;
    v_code   JSONB;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM inventory.products p WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF (SELECT COUNT(*) FROM jsonb_array_elements(p_units) e WHERE (e ->> 'is_base')::BOOLEAN) <> 1
       OR EXISTS (SELECT 1 FROM jsonb_array_elements(p_units) e WHERE (e ->> 'is_base')::BOOLEAN AND (e ->> 'factor')::NUMERIC <> 1)
    THEN
        RAISE EXCEPTION 'inventory.unit.base-required' USING ERRCODE = 'P0001';
    END IF;

    -- The base is the smallest unit sold: every other presentation holds more than one of it.
    IF EXISTS (SELECT 1 FROM jsonb_array_elements(p_units) e WHERE NOT COALESCE((e ->> 'is_base')::BOOLEAN, FALSE) AND (e ->> 'factor')::NUMERIC <= 1) THEN
        RAISE EXCEPTION 'inventory.unit.factor-invalid' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM jsonb_array_elements(p_units) e
        WHERE NOT EXISTS (
            SELECT 1 FROM inventory.units u
            WHERE u.id = (e ->> 'unit_id')::UUID AND (u.tenant_id IS NULL OR u.tenant_id = p_tenant_id)
        )
    ) THEN
        RAISE EXCEPTION 'inventory.unit.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- The base moves last in the diff's eye: free its slot first, so a swap of base never trips the one-base index.
    UPDATE inventory.product_units pu SET is_base = FALSE
    WHERE pu.product_id = p_product_id AND pu.tenant_id = p_tenant_id AND pu.is_base;

    -- Codes are the row's whole list on every save; the unique index refuses one another row of this business holds.
    DELETE FROM inventory.sku_codes sc WHERE sc.tenant_id = p_tenant_id AND sc.product_unit_id IN (
        SELECT pu.id FROM inventory.product_units pu WHERE pu.product_id = p_product_id AND pu.tenant_id = p_tenant_id
    );

    FOR v_row IN SELECT * FROM jsonb_array_elements(p_units) LOOP
        v_id := NULLIF(v_row ->> 'id', '')::UUID;
        IF v_id IS NULL THEN
            INSERT INTO inventory.product_units (tenant_id, product_id, unit_id, factor)
            VALUES (p_tenant_id, p_product_id, (v_row ->> 'unit_id')::UUID, (v_row ->> 'factor')::NUMERIC)
            RETURNING id INTO v_id;
        END IF;

        UPDATE inventory.product_units pu
        SET unit_id = (v_row ->> 'unit_id')::UUID,
            factor = (v_row ->> 'factor')::NUMERIC,
            is_base = COALESCE((v_row ->> 'is_base')::BOOLEAN, FALSE),
            allow_fraction = COALESCE((v_row ->> 'allow_fraction')::BOOLEAN, FALSE),
            sellable = COALESCE((v_row ->> 'sellable')::BOOLEAN, TRUE),
            pos_visible = COALESCE((v_row ->> 'pos_visible')::BOOLEAN, TRUE),
            is_sale_default = COALESCE((v_row ->> 'is_sale_default')::BOOLEAN, FALSE),
            is_purchase_default = COALESCE((v_row ->> 'is_purchase_default')::BOOLEAN, FALSE),
            price = (v_row ->> 'price')::NUMERIC,
            is_active = TRUE
        WHERE pu.id = v_id AND pu.product_id = p_product_id AND pu.tenant_id = p_tenant_id;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'inventory.unit.not-found' USING ERRCODE = 'P0001';
        END IF;
        v_kept := v_kept || v_id;

        FOR v_code IN SELECT * FROM jsonb_array_elements(CASE WHEN jsonb_typeof(v_row -> 'codes') = 'array' THEN v_row -> 'codes' ELSE '[]'::jsonb END) LOOP
            IF NOT EXISTS (
                SELECT 1 FROM inventory.product_skus s
                WHERE s.id = (v_code ->> 'sku_id')::UUID AND s.product_id = p_product_id AND s.tenant_id = p_tenant_id
            ) THEN
                RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
            END IF;
            BEGIN
                INSERT INTO inventory.sku_codes (tenant_id, code, sku_id, product_unit_id)
                VALUES (p_tenant_id, TRIM(v_code ->> 'code'), (v_code ->> 'sku_id')::UUID, v_id);
            EXCEPTION WHEN unique_violation THEN
                RAISE EXCEPTION 'inventory.code.duplicate' USING ERRCODE = 'P0001';
            END;
        END LOOP;
    END LOOP;

    -- A presentation a line still names stays, inactive: the old order keeps reading it.
    FOR v_gone IN
        SELECT pu.id FROM inventory.product_units pu
        WHERE pu.product_id = p_product_id AND pu.tenant_id = p_tenant_id AND pu.id <> ALL (v_kept)
    LOOP
        BEGIN
            DELETE FROM inventory.product_units pu WHERE pu.id = v_gone;
        EXCEPTION WHEN foreign_key_violation THEN
            UPDATE inventory.product_units pu SET is_active = FALSE, is_sale_default = FALSE, is_purchase_default = FALSE WHERE pu.id = v_gone;
        END;
    END LOOP;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_replace_product_units(uuid,uuid,jsonb) IS 'Replaces a product''s presentations and their codes, diffed by id, exactly one base at factor 1, every other above it (INV-023). A presentation a line names is deactivated, not deleted. Raises inventory.unit.base-required, inventory.unit.factor-invalid, inventory.unit.not-found, inventory.code.duplicate.';
