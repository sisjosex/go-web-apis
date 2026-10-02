-- TRACK-032 step 2: the tracking half of app access — who an account is linked to.
--
-- The account and its access level are granted first by tenancy.sp_grant_app_access in the main
-- database (it refuses web accounts and other app levels, D4); these SPs then write the link. They
-- never read tenancy.*, so they run unchanged in a dedicated tenant database.
--
-- A rider may have several guardian accounts (D3): each is a rider_contacts row, relation
-- 'guardian', with user_id set — exactly what fn_guardian_scope already reads (every guardian row,
-- not only the primary one). Adding the same account twice is a no-op; a guardian contact typed in
-- the rider form with the same email gains the user_id instead of a second row.
--
-- fn_user_linked answers whether anything in the tenant still links an account, so Go can end the
-- app membership after the last unlink. Removal is rare; the extra round trip is not worth a
-- combined signature on every unlink SP.

CREATE FUNCTION tracking.fn_user_linked(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT EXISTS (
        SELECT 1 FROM tracking.rider_contacts rc
        INNER JOIN tracking.riders r ON r.id = rc.rider_id
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE rc.user_id = p_user_id AND rc.relation = 'guardian' AND o.tenant_id = p_tenant_id
    ) OR EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.user_id = p_user_id AND d.tenant_id = p_tenant_id
    ) OR EXISTS (
        SELECT 1 FROM tracking.organization_members m
        INNER JOIN tracking.organizations o ON o.id = m.organization_id
        WHERE m.user_id = p_user_id AND o.tenant_id = p_tenant_id
    );
$$;

COMMENT ON FUNCTION tracking.fn_user_linked(UUID, UUID) IS
'Whether a rider guardianship, a driver or an organization membership in the tenant still names the account (TRACK-032)';

-- The rider guard every guardian SP shares: in the tenant and in the caller's organization scope,
-- else rider.not-found (out of scope reads as not found, TRACK-015).
CREATE FUNCTION tracking.fn_require_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID
)
RETURNS VOID
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;

CREATE FUNCTION tracking.sp_list_rider_guardians(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_scope_user_id UUID
)
RETURNS TABLE(
    user_id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    is_primary BOOLEAN
)
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id);

    RETURN QUERY
    SELECT g.* FROM (
        SELECT DISTINCT ON (rc.user_id)
            rc.user_id,
            CAST(COALESCE(u.email, rc.email) AS VARCHAR) AS email,
            CAST(COALESCE(u.first_name, rc.name) AS VARCHAR) AS first_name,
            CAST(u.last_name AS VARCHAR) AS last_name,
            CAST(COALESCE(rc.phone, u.phone) AS VARCHAR) AS phone,
            rc.is_primary
        FROM tracking.rider_contacts rc
        -- auth.users sits beside us on a shared database; elsewhere the contact's own copy answers.
        LEFT JOIN auth.users u ON u.id = rc.user_id AND u.deleted_at IS NULL
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id IS NOT NULL
        ORDER BY rc.user_id, rc.is_primary DESC
    ) g
    ORDER BY g.is_primary DESC, g.first_name, g.last_name;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_rider_guardians(UUID, UUID, UUID) IS
'The guardian accounts of a rider (D3); raises tracking.rider.not-found outside the tenant or the caller''s organization scope';

CREATE FUNCTION tracking.sp_rider_guardian_add(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_user_id UUID,
    p_name VARCHAR(255),
    p_email VARCHAR(255),
    p_phone VARCHAR(50),
    p_scope_user_id UUID
)
RETURNS VOID
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id);

    IF EXISTS (
        SELECT 1 FROM tracking.rider_contacts rc
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
    ) THEN
        RETURN;
    END IF;

    -- The guardian the rider form already names by this email becomes the account's row.
    UPDATE tracking.rider_contacts rc SET
        user_id = p_user_id,
        updated_at = CURRENT_TIMESTAMP
    WHERE rc.id = (
        SELECT c.id FROM tracking.rider_contacts c
        WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.user_id IS NULL
          AND LOWER(c.email) = LOWER(TRIM(p_email))
        ORDER BY c.is_primary DESC
        LIMIT 1
    );
    IF FOUND THEN
        RETURN;
    END IF;

    INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
    VALUES (
        p_rider_id, 'guardian',
        NULLIF(TRIM(p_name), ''), NULLIF(TRIM(p_phone), ''), NULLIF(LOWER(TRIM(p_email)), ''),
        p_user_id,
        NOT EXISTS (
            SELECT 1 FROM tracking.rider_contacts c
            WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.is_primary
        )
    );
END;
$$;

COMMENT ON FUNCTION tracking.sp_rider_guardian_add(UUID, UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID) IS
'Links a guardian account to a rider (idempotent); the access level was granted by tenancy.sp_grant_app_access first';

-- The primary guardian keeps its name and phone as a plain contact; any other row was the account's
-- and goes with it.
CREATE FUNCTION tracking.sp_rider_guardian_remove(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_user_id UUID,
    p_scope_user_id UUID
)
RETURNS VOID
LANGUAGE plpgsql AS $$
DECLARE
    v_count INT;
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id);

    DELETE FROM tracking.rider_contacts rc
    WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
      AND NOT rc.is_primary;
    GET DIAGNOSTICS v_count = ROW_COUNT;

    UPDATE tracking.rider_contacts rc SET user_id = NULL, updated_at = CURRENT_TIMESTAMP
    WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
      AND rc.is_primary;

    IF v_count = 0 AND NOT FOUND THEN
        RAISE EXCEPTION 'rider.guardian-not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$;

COMMENT ON FUNCTION tracking.sp_rider_guardian_remove(UUID, UUID, UUID, UUID) IS
'Unlinks a guardian account from a rider; raises tracking.rider.guardian-not-found when it was not linked';

-- Replaces the user_id paths of sp_update_driver, which accepted any account. Answers the account
-- it replaced, so Go can end that one's membership when nothing else links it.
CREATE FUNCTION tracking.sp_driver_account_set(
    p_tenant_id UUID,
    p_driver_id UUID,
    p_user_id UUID
)
RETURNS UUID
LANGUAGE plpgsql AS $$
DECLARE
    v_previous UUID;
BEGIN
    SELECT d.user_id INTO v_previous
    FROM tracking.drivers d
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.drivers d SET user_id = p_user_id, updated_at = CURRENT_TIMESTAMP
    WHERE d.id = p_driver_id;

    RETURN NULLIF(v_previous, p_user_id);
END;
$$;

COMMENT ON FUNCTION tracking.sp_driver_account_set(UUID, UUID, UUID) IS
'Links (or with NULL clears) a driver''s app account and answers the account it replaced; raises tracking.driver.not-found or tracking.driver.user-already-linked';
