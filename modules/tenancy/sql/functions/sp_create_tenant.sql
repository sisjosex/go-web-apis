CREATE OR REPLACE FUNCTION tenancy.sp_create_tenant(p_slug character varying, p_name character varying, p_creator_user_id uuid, p_database_url text DEFAULT NULL::text, p_schema_name character varying DEFAULT 'public'::character varying, p_settings jsonb DEFAULT '{}'::jsonb)
 RETURNS TABLE(id uuid, slug character varying, name character varying, database_url text, schema_name character varying, is_active boolean, is_suspended boolean, suspended_reason text, settings jsonb, created_at timestamp with time zone, updated_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_slug VARCHAR;
    v_name VARCHAR;
    v_tenant_id UUID;
BEGIN
    -- Validate creator user ID
    IF p_creator_user_id IS NULL THEN
        RAISE EXCEPTION 'tenant.creator.required' USING ERRCODE = 'T0011';
    END IF;

    -- Verify creator user exists
    IF NOT EXISTS (SELECT 1 FROM auth.users u WHERE u.id = p_creator_user_id) THEN
        RAISE EXCEPTION 'user.not-found' USING ERRCODE = 'U0002';
    END IF;
    -- Validate and normalize slug (lowercase, trim)
    v_slug := LOWER(TRIM(p_slug));
    
    IF v_slug = '' OR v_slug IS NULL THEN
        RAISE EXCEPTION 'tenant.slug.required' USING ERRCODE = 'T0001';
    END IF;

    -- Validate slug format (only lowercase letters, numbers, hyphens)
    IF v_slug !~ '^[a-z0-9]+(?:-[a-z0-9]+)*$' THEN
        RAISE EXCEPTION 'tenant.slug.invalid' USING ERRCODE = 'T0002';
    END IF;

    -- Validate name
    v_name := TRIM(p_name);
    IF v_name = '' OR v_name IS NULL THEN
        RAISE EXCEPTION 'tenant.name.required' USING ERRCODE = 'T0003';
    END IF;

    -- Check if slug already exists
    IF EXISTS (SELECT 1 FROM tenancy.tenants WHERE tenants.slug = v_slug) THEN
        RAISE EXCEPTION 'tenant.slug.already-exists' USING ERRCODE = 'T0004';
    END IF;

    -- Insert new tenant
    INSERT INTO tenancy.tenants (slug, name, database_url, schema_name, settings)
    VALUES (v_slug, v_name, p_database_url, p_schema_name, p_settings)
    RETURNING tenants.id INTO v_tenant_id;

    -- Add creator as owner of the tenant
    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role, is_active)
    VALUES (v_tenant_id, p_creator_user_id, 'owner', TRUE);

    -- Return the created tenant
    RETURN QUERY
    SELECT 
        t.id,
        t.slug,
        t.name,
        t.database_url,
        t.schema_name,
        t.is_active,
        t.is_suspended,
        t.suspended_reason,
        t.settings,
        t.created_at,
        t.updated_at
    FROM tenancy.tenants t
    WHERE t.id = v_tenant_id;
END;
$function$;
