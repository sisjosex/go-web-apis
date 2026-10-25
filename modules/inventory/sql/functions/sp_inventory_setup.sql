CREATE OR REPLACE FUNCTION inventory.sp_inventory_setup(p_tenant_id uuid)
 RETURNS TABLE(products boolean, categories boolean)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        EXISTS (SELECT 1 FROM inventory.products p WHERE p.tenant_id = p_tenant_id),
        EXISTS (SELECT 1 FROM inventory.product_categories c WHERE c.tenant_id = p_tenant_id AND c.deleted_at IS NULL);
$function$;

COMMENT ON FUNCTION inventory.sp_inventory_setup(uuid) IS 'The first-run checklist''s inventory steps (TENANCY-008): whether the tenant has any product and any live category yet';
