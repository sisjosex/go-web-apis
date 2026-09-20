-- TRACK-017 step 2 (D2): a `portal` guardian sees the riders it is a contact of, and nothing else.
--
-- The scope is resolved once per call, inside the SP, from p_guardian_user_id: the caller's user id
-- when their access level is `portal`, and NULL for everyone else, who read exactly as before. Same
-- shape as fn_organization_scope (TRACK-015 D1) and the same cost — the access row the tenant
-- middleware already fetched, then the SP, no third round-trip.
--
-- It differs from fn_organization_scope in one way, deliberately: an empty scope is an empty array,
-- not a refusal. A parent the school has not linked to a rider yet is a normal state the app has
-- copy for, not a misconfiguration. `= ANY('{}')` matches nothing, so the reads answer 200 with an
-- empty list.
--
-- rider_contacts has no tenant_id, so the tenant guard is the join out to riders and their
-- organization. The lookup is served by the partial idx_rider_contacts_user_id; a rider with two
-- guardians is two rows and both guardians hold it in scope. DISTINCT because a rider may carry more
-- than one guardian contact row for the same user.

CREATE OR REPLACE FUNCTION tracking.fn_guardian_scope(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS UUID[]
LANGUAGE plpgsql
STABLE
AS $$
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
      AND rc.relation = 'guardian'
      AND o.tenant_id = p_tenant_id;

    RETURN COALESCE(v_scope, ARRAY[]::UUID[]);
END;
$$;

COMMENT ON FUNCTION tracking.fn_guardian_scope(UUID, UUID) IS
'The rider ids a guardian account is a contact of in this tenant; an empty array when the school has linked it to none, never a refusal (TRACK-017 D2)';
