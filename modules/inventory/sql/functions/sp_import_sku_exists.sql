CREATE OR REPLACE FUNCTION inventory.sp_import_sku_exists(p_tenant_id uuid, p_sku character varying)
 RETURNS TABLE(product_sku_taken boolean, variant_sku_taken boolean)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.sp_import_sku_exists(uuid,character varying) IS 'Whether the tenant already has a product (product_sku_taken) or a combination (variant_sku_taken) with this code, so the dry run previews the duplicate the create SPs would refuse. Read-only (INV-017 D1).';
