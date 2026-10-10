CREATE OR REPLACE FUNCTION inventory.fn_resolve_sku(p_tenant_id uuid, p_product_id uuid, p_sku_id uuid DEFAULT NULL::uuid)
 RETURNS uuid
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_sku_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM inventory.products p
        WHERE p.id = p_product_id
          AND p.tenant_id = p_tenant_id
    )
    THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_sku_id IS NULL
    THEN
        SELECT s.id INTO v_sku_id
        FROM inventory.product_skus s
        WHERE s.product_id = p_product_id
          AND s.is_default;

        -- Every product gets a default SKU by 20260812000003; missing one means
        -- the product predates the backfill, which is a product-level problem.
        IF v_sku_id IS NULL
        THEN
            RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
        END IF;
    ELSE
        SELECT s.id INTO v_sku_id
        FROM inventory.product_skus s
        WHERE s.id = p_sku_id
          AND s.product_id = p_product_id
          AND s.tenant_id = p_tenant_id;

        IF v_sku_id IS NULL
        THEN
            RAISE EXCEPTION 'inventory.sku.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN v_sku_id;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_resolve_sku(uuid,uuid,uuid) IS 'Resolves the SKU a stock operation applies to: the product default when p_sku_id is NULL, otherwise p_sku_id after checking it belongs to that product and that tenant. Raises product.not-found for a product outside the tenant and inventory.sku.not-found for a SKU of another product.';
