CREATE OR REPLACE FUNCTION inventory.fn_requires_sku(p_tenant_id uuid, p_product_id uuid)
 RETURNS boolean
 LANGUAGE sql
 STABLE
AS $function$
    SELECT EXISTS (
        SELECT 1
        FROM inventory.products p
        WHERE p.id = p_product_id
          AND p.tenant_id = p_tenant_id
          AND p.stock_by_variant
          AND EXISTS (
              SELECT 1
              FROM inventory.product_skus s
              WHERE s.product_id = p.id
                AND NOT s.is_default
          )
    );
$function$;

COMMENT ON FUNCTION inventory.fn_requires_sku(uuid,uuid) IS 'Whether a stock write against this product must name a SKU: the product is stocked by variant and its combinations have been generated. FALSE for a product of another tenant or one that does not exist — the caller''s own not-found check owns that case.';
