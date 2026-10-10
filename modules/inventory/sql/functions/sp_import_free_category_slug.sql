CREATE OR REPLACE FUNCTION inventory.sp_import_free_category_slug(p_tenant_id uuid, p_base character varying, p_limit integer DEFAULT 20)
 RETURNS TABLE(slug character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_candidate VARCHAR := p_base;
    v_suffix    INT;
BEGIN
    -- product_categories.slug is UNIQUE per tenant, deleted rows included, so
    -- two categories named the same under different parents need base, base-2…
    FOR v_suffix IN 1..p_limit
    LOOP
        IF v_suffix > 1
        THEN
            v_candidate := CAST(p_base || '-' || v_suffix AS VARCHAR);
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM inventory.product_categories c
            WHERE c.tenant_id = p_tenant_id
              AND c.slug      = v_candidate
        )
        THEN
            RETURN QUERY SELECT v_candidate;
            RETURN;
        END IF;
    END LOOP;

    -- Every candidate taken: the base comes back and sp_create_category refuses
    -- it, which is the row error the caller already reports.
    RETURN QUERY SELECT CAST(p_base AS VARCHAR);
END;
$function$;

COMMENT ON FUNCTION inventory.sp_import_free_category_slug(uuid,character varying,integer) IS 'The first of p_base, p_base-2 … p_base-<p_limit> the tenant is not using as a category slug; p_base itself when every one is taken. Read-only (INV-017 D1).';
