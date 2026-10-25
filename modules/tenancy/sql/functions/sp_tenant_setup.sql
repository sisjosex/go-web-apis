CREATE OR REPLACE FUNCTION tenancy.sp_tenant_setup(p_tenant_id uuid)
 RETURNS TABLE(business boolean, team boolean, verticals text[])
 LANGUAGE sql
 STABLE
AS $function$
    -- TENANCY-008: the checklist's shared steps and the business types that order it, in one call.
    -- `business` is done once a general.* setting was saved (TENANCY-007); `team` once someone besides
    -- the creator works here — a guardian (portal) is a client, not the team.
    SELECT
        EXISTS (
            SELECT 1 FROM jsonb_object_keys(COALESCE(t.settings, '{}'::jsonb)) AS k(key)
            WHERE k.key LIKE 'general.%'
        ),
        (
            SELECT COUNT(*) FROM tenancy.tenant_users tu
            WHERE tu.tenant_id = t.id AND tu.is_active AND tu.role <> 'portal'
        ) > 1,
        CAST(t.verticals AS TEXT[])
    FROM tenancy.tenants t
    WHERE t.id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tenancy.sp_tenant_setup(uuid) IS 'The first-run checklist''s shared steps (TENANCY-008): whether a general setting was saved, whether a second member works here, and the business types that order the list';
