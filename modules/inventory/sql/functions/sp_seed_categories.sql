CREATE OR REPLACE FUNCTION inventory.sp_seed_categories(p_tenant_id uuid, p_names character varying[])
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_name    VARCHAR;
    v_slug    VARCHAR;
    v_order   INT := 0;
    v_created INT := 0;
BEGIN
    IF p_tenant_id IS NULL THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    FOREACH v_name IN ARRAY COALESCE(p_names, ARRAY[]::VARCHAR[])
    LOOP
        v_name := TRIM(v_name);
        CONTINUE WHEN v_name = '';
        v_order := v_order + 1;

        -- Idempotent by name: a category the tenant already has, under any parent, is left alone.
        CONTINUE WHEN EXISTS (
            SELECT 1
            FROM inventory.product_categories c
            WHERE c.tenant_id   = p_tenant_id
              AND LOWER(c.name) = LOWER(v_name)
              AND c.deleted_at IS NULL
        );

        v_slug := TRIM(BOTH '-' FROM REGEXP_REPLACE(
            LOWER(TRANSLATE(v_name, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun')), '[^a-z0-9]+', '-', 'g'));
        IF v_slug = '' THEN
            v_slug := 'category';
        END IF;
        SELECT f.slug INTO v_slug FROM inventory.sp_import_free_category_slug(p_tenant_id, v_slug) f;

        INSERT INTO inventory.product_categories (tenant_id, name, slug, display_order)
        VALUES (p_tenant_id, v_name, v_slug, v_order);
        v_created := v_created + 1;
    END LOOP;

    RETURN v_created;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_seed_categories(uuid,character varying[]) IS 'Creates the top-level categories of p_names the tenant does not have yet (live, case-insensitive name) and answers how many it created; a business type''s starter catalogue (TENANCY-006 D2), safe to repeat';
