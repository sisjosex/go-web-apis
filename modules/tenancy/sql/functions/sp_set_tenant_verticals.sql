CREATE OR REPLACE FUNCTION tenancy.sp_set_tenant_verticals(p_tenant_id uuid, p_verticals character varying[], p_primary_module character varying, p_fallback_module character varying, p_capabilities character varying[], p_restrict boolean)
 RETURNS TABLE(verticals text[], primary_module character varying, settings jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_settings JSONB;
    v_caps     JSONB;
BEGIN
    SELECT COALESCE(t.settings, '{}'::jsonb) INTO v_settings
    FROM tenancy.tenants t
    WHERE t.id = p_tenant_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'T0005';
    END IF;

    -- TENANCY-006 D3: a tenant without a stored list has every capability on, so a re-pick leaves it
    -- that way; a stored list gains the types' capabilities and never loses one; the sign-up
    -- (p_restrict) stores the types' own list.
    IF v_settings ? 'capabilities' THEN
        SELECT COALESCE(jsonb_agg(DISTINCT c.code ORDER BY c.code), '[]'::jsonb) INTO v_caps
        FROM (
            SELECT jsonb_array_elements_text(v_settings -> 'capabilities') AS code
            UNION
            SELECT UNNEST(COALESCE(p_capabilities, ARRAY[]::VARCHAR[]))
        ) c;
    ELSIF p_restrict THEN
        SELECT COALESCE(jsonb_agg(DISTINCT c.code ORDER BY c.code), '[]'::jsonb) INTO v_caps
        FROM UNNEST(COALESCE(p_capabilities, ARRAY[]::VARCHAR[])) AS c(code);
    END IF;

    -- The API validated the codes against the registry and p_primary_module against the enabled modules.
    -- An empty p_verticals keeps the stored types (a change of main module only); the main module is the
    -- one sent, else the one kept, else the first type's (APP-015 D2).
    RETURN QUERY
    UPDATE tenancy.tenants t
    SET verticals      = CASE WHEN CARDINALITY(p_verticals) > 0 THEN p_verticals ELSE t.verticals END,
        primary_module = COALESCE(NULLIF(p_primary_module, ''), t.primary_module, NULLIF(p_fallback_module, '')),
        settings       = CASE WHEN v_caps IS NULL THEN t.settings
                              ELSE COALESCE(t.settings, '{}'::jsonb) || jsonb_build_object('capabilities', v_caps) END
    WHERE t.id = p_tenant_id
    RETURNING CAST(t.verticals AS TEXT[]), CAST(t.primary_module AS VARCHAR), COALESCE(t.settings, '{}'::jsonb);
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_set_tenant_verticals(uuid,character varying[],character varying,character varying,character varying[],boolean) IS 'Stores a tenant''s business types, its main module (sent, else kept, else the fallback) and adds the types'' capabilities to settings.capabilities (TENANCY-006 D2-D3, APP-015 D2); answers all three';
