CREATE OR REPLACE FUNCTION tenancy.sp_get_tenant_by_slug(p_slug character varying)
 RETURNS TABLE(id uuid, slug character varying, name character varying, database_url text, schema_name character varying, is_active boolean, is_suspended boolean, created_at timestamp with time zone, settings jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_slug VARCHAR;
BEGIN
    v_slug := LOWER(TRIM(p_slug));
    
    IF v_slug = '' OR v_slug IS NULL THEN
        RAISE EXCEPTION 'tenant.slug.required' USING ERRCODE = 'T0001';
    END IF;

    RETURN QUERY
    SELECT 
        t.id,
        t.slug,
        t.name,
        t.database_url,
        t.schema_name,
        t.is_active,
        t.is_suspended,
        t.created_at,
        COALESCE(t.settings, '{}'::jsonb)
    FROM tenancy.tenants t
    WHERE t.slug = v_slug;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_get_tenant_by_slug(character varying) IS 'A tenant by slug with its database and its stored settings, for the tenant chains and their shared cache (TENANCY-007 D1)';
