CREATE OR REPLACE FUNCTION inventory.sp_import_find_category(p_tenant_id uuid, p_name character varying, p_parent_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(category_id uuid)
 LANGUAGE plpgsql
 STABLE
AS $function$
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
$function$;

COMMENT ON FUNCTION inventory.sp_import_find_category(uuid,character varying,uuid) IS 'The oldest live category of the tenant with this name, case-insensitive; p_parent_id narrows it to the children of one category. No row when there is none. Read-only (INV-017 D1).';
