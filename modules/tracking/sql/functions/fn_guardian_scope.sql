CREATE OR REPLACE FUNCTION tracking.fn_guardian_scope(p_tenant_id uuid, p_user_id uuid)
 RETURNS uuid[]
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    SELECT ARRAY_AGG(DISTINCT rc.rider_id)
    INTO v_scope
    FROM tracking.rider_contacts rc
    INNER JOIN tracking.riders r ON r.id = rc.rider_id
    -- The tenant guard: a guardianship over another tenant's rider is not a scope here.
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE rc.user_id = p_user_id
      -- MOBILE-039: an adult passenger is their own contact ('self'), and reads their own day as a guardian does.
      AND rc.relation IN ('guardian', 'self')
      AND o.tenant_id = p_tenant_id;

    RETURN COALESCE(v_scope, ARRAY[]::UUID[]);
END;
$function$;

COMMENT ON FUNCTION tracking.fn_guardian_scope(uuid,uuid) IS 'The rider ids a guardian or self account is a contact of in this tenant (MOBILE-039); an empty array when the school has linked it to none, never a refusal (TRACK-017 D2)';
