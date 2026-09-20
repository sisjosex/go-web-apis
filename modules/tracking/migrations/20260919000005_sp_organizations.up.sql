-- TRACK-005 step 2: the organizations slice — CRUD on the purchasing shape, plus the membership
-- endpoints that put tenant users on an organization (D3).
--
-- Members are read with their identity joined from `auth.users`, the same join `users.sp_list_users`
-- makes, so the app renders a name and an email without a second round trip per row. Tracking only
-- ever runs against a tenant database, which carries `auth.*`; nothing here touches `tenancy.*`,
-- which a tenant database does not have.
--
-- Deleting an organization that still has riders is refused rather than cascading: the riders would
-- go with it (ON DELETE CASCADE on riders.organization_id), which is never what the click meant.

-- ===========================================================================
-- Organizations CRUD
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_organization(
    p_tenant_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    INSERT INTO tracking.organizations (tenant_id, kind, name, timezone, is_active)
    VALUES (
        p_tenant_id,
        COALESCE(NULLIF(TRIM(p_kind), ''), 'other'),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_timezone), ''), 'UTC'),
        COALESCE(p_is_active, true)
    )
    RETURNING
        tracking.organizations.id,
        tracking.organizations.tenant_id,
        CAST(tracking.organizations.kind AS VARCHAR),
        CAST(tracking.organizations.name AS VARCHAR),
        CAST(tracking.organizations.timezone AS VARCHAR),
        tracking.organizations.is_active,
        tracking.organizations.created_at,
        tracking.organizations.updated_at;
END;
$$ LANGUAGE plpgsql;

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field.
CREATE FUNCTION tracking.sp_update_organization(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_kind VARCHAR(20),
    p_name VARCHAR(255),
    p_timezone VARCHAR(64),
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.organizations o SET
        kind      = COALESCE(NULLIF(TRIM(p_kind), ''), o.kind),
        name      = COALESCE(NULLIF(TRIM(p_name), ''), o.name),
        timezone  = COALESCE(NULLIF(TRIM(p_timezone), ''), o.timezone),
        is_active = COALESCE(p_is_active, o.is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    RETURNING
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_organizations(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_kind      VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page      INT     DEFAULT 1,
    p_page_size INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.organizations o
    WHERE o.tenant_id = p_tenant_id
      AND (p_search IS NULL OR p_search = '' OR o.name ILIKE '%' || p_search || '%')
      AND (p_kind IS NULL OR p_kind = '' OR o.kind = p_kind)
      AND (p_is_active IS NULL OR o.is_active = p_is_active)
    -- Alphabetical, so the page boundary follows the order the reader sees; o.id breaks ties.
    ORDER BY o.name ASC, o.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_organizations(UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT) IS
'One page of a tenant''s organizations by name, optionally narrowed by a name search, kind and status; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_organization(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    kind VARCHAR,
    name VARCHAR,
    timezone VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- Members go with the organization (ON DELETE CASCADE); riders never do.
CREATE FUNCTION tracking.sp_delete_organization(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.riders r WHERE r.organization_id = p_organization_id
    ) THEN
        RAISE EXCEPTION 'organization.has-riders' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- Members
-- ===========================================================================
CREATE FUNCTION tracking.sp_list_organization_members(
    p_tenant_id UUID,
    p_organization_id UUID
)
RETURNS TABLE(
    user_id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    role VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        m.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(m.role AS VARCHAR)
    FROM tracking.organization_members m
    -- A member whose user row was deleted still holds the seat; the app shows the id rather than
    -- dropping the row silently, so an operator can see what to remove.
    LEFT JOIN auth.users u ON u.id = m.user_id AND u.deleted_at IS NULL
    WHERE m.organization_id = p_organization_id
    ORDER BY u.last_name ASC, u.first_name ASC, m.user_id ASC;
END;
$$ LANGUAGE plpgsql;

-- Upsert: PUT on a member that is already there changes the role, so the app needs no "is this an
-- add or an edit" branch.
CREATE FUNCTION tracking.sp_upsert_organization_member(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_user_id UUID,
    p_role VARCHAR(20)
)
RETURNS TABLE(
    user_id UUID,
    email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    role VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM auth.users u WHERE u.id = p_user_id AND u.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'organization-member.user-not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.organization_members (organization_id, user_id, role)
    VALUES (p_organization_id, p_user_id, p_role)
    ON CONFLICT ON CONSTRAINT uk_organization_member DO UPDATE
        SET role = EXCLUDED.role,
            updated_at = CURRENT_TIMESTAMP;

    RETURN QUERY
    SELECT
        m.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(m.role AS VARCHAR)
    FROM tracking.organization_members m
    INNER JOIN auth.users u ON u.id = m.user_id
    WHERE m.organization_id = p_organization_id AND m.user_id = p_user_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_delete_organization_member(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_user_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.organization_members m
    WHERE m.organization_id = p_organization_id AND m.user_id = p_user_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    IF deleted_count = 0 THEN
        RAISE EXCEPTION 'organization-member.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN true;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_organization_member(UUID, UUID, UUID) IS
'Takes a user off an organization; raises tracking.organization-member.not-found when they were not on it, and leaves the user account untouched';
