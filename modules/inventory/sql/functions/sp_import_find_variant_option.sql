CREATE OR REPLACE FUNCTION inventory.sp_import_find_variant_option(p_tenant_id uuid, p_product_id uuid, p_axis character varying, p_option character varying)
 RETURNS TABLE(option_id uuid)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.sp_import_find_variant_option(uuid,uuid,character varying,character varying) IS 'The option of one inventory axis of a tenant''s product, matched case-insensitively, that an imported picture hangs off (INV-016 D7). No row when the product is not the tenant''s or has no such option. Read-only (INV-017 D1).';
