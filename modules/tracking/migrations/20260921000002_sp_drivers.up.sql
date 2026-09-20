-- TRACK-006 step 2: the drivers slice — CRUD on the purchasing shape, the same one organizations,
-- vehicles and companies already answer with.
--
-- Every read carries `company_name` and `user_email` joined here, so a list page renders the carrier
-- and the linked account without a second round trip per row. `auth.users` is joined the way
-- sp_list_organization_members already joins it: tracking only ever runs against a tenant database,
-- which carries `auth.*`, and nothing here touches `tenancy.*`, which a tenant database does not have.
--
-- The licence and the account are checked before the write rather than left to the unique
-- constraints: a 23505 tells Go that something collided, not which of the two did, and the app shows
-- a different message for each.
--
-- `company_id` is immutable, so sp_update_driver does not take one: moving a driver between carriers
-- would silently re-scope the routes that name them as default driver. Delete and re-create is the
-- honest way to do that, and the has-routes check makes the consequence visible first.

-- ===========================================================================
-- Drivers CRUD
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_driver(
    p_tenant_id UUID,
    p_company_id UUID,
    p_user_id UUID,
    p_first_name VARCHAR(255),
    p_last_name VARCHAR(255),
    p_phone VARCHAR(50),
    p_license_number VARCHAR(100),
    p_license_class VARCHAR(50),
    p_license_expires_on DATE,
    p_status VARCHAR(20)
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    company_id UUID,
    company_name VARCHAR,
    user_id UUID,
    user_email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    license_number VARCHAR,
    license_class VARCHAR,
    license_expires_on DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_driver_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.license_number = TRIM(p_license_number)
    ) THEN
        RAISE EXCEPTION 'driver.license-already-exists' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.drivers (
        tenant_id, company_id, user_id, first_name, last_name, phone,
        license_number, license_class, license_expires_on, status
    )
    VALUES (
        p_tenant_id,
        p_company_id,
        p_user_id,
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_phone), ''),
        TRIM(p_license_number),
        NULLIF(TRIM(p_license_class), ''),
        p_license_expires_on,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING tracking.drivers.id INTO v_driver_id;

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.id = v_driver_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_driver(UUID, UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR) IS
'Creates a driver for one of the tenant''s carriers; raises tracking.company.not-found, tracking.driver.license-already-exists or tracking.driver.user-already-linked';

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field. The one
-- field that must also be clearable is the account link, which the driver screen unlinks explicitly:
-- p_clear_user_id says "set it to NULL", which no value of p_user_id can say on its own.
CREATE FUNCTION tracking.sp_update_driver(
    p_tenant_id UUID,
    p_driver_id UUID,
    p_user_id UUID,
    p_clear_user_id BOOLEAN,
    p_first_name VARCHAR(255),
    p_last_name VARCHAR(255),
    p_phone VARCHAR(50),
    p_license_number VARCHAR(100),
    p_license_class VARCHAR(50),
    p_license_expires_on DATE,
    p_status VARCHAR(20)
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    company_id UUID,
    company_name VARCHAR,
    user_id UUID,
    user_email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    license_number VARCHAR,
    license_class VARCHAR,
    license_expires_on DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_license_number IS NOT NULL AND TRIM(p_license_number) <> '' AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id
          AND d.license_number = TRIM(p_license_number)
          AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.license-already-exists' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.drivers d SET
        user_id            = CASE WHEN COALESCE(p_clear_user_id, false) THEN NULL
                                  ELSE COALESCE(p_user_id, d.user_id) END,
        first_name         = COALESCE(NULLIF(TRIM(p_first_name), ''), d.first_name),
        last_name          = COALESCE(NULLIF(TRIM(p_last_name), ''), d.last_name),
        phone              = COALESCE(NULLIF(TRIM(p_phone), ''), d.phone),
        license_number     = COALESCE(NULLIF(TRIM(p_license_number), ''), d.license_number),
        license_class      = COALESCE(NULLIF(TRIM(p_license_class), ''), d.license_class),
        license_expires_on = COALESCE(p_license_expires_on, d.license_expires_on),
        status             = COALESCE(NULLIF(TRIM(p_status), ''), d.status),
        updated_at         = CURRENT_TIMESTAMP
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.id = p_driver_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_update_driver(UUID, UUID, UUID, BOOLEAN, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, DATE, VARCHAR) IS
'Updates a driver; company_id is immutable, p_clear_user_id unlinks the account, and a licence or account already in use raises tracking.driver.license-already-exists / tracking.driver.user-already-linked';

CREATE FUNCTION tracking.sp_list_drivers(
    p_tenant_id  UUID,
    p_search     VARCHAR DEFAULT NULL,
    p_company_id UUID    DEFAULT NULL,
    p_status     VARCHAR DEFAULT NULL,
    p_page       INT     DEFAULT 1,
    p_page_size  INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    company_id UUID,
    company_name VARCHAR,
    user_id UUID,
    user_email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    license_number VARCHAR,
    license_class VARCHAR,
    license_expires_on DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR d.company_id = p_company_id)
      AND (p_status IS NULL OR p_status = '' OR d.status = p_status)
      AND (p_search IS NULL OR p_search = ''
           OR d.first_name ILIKE '%' || p_search || '%'
           OR d.last_name ILIKE '%' || p_search || '%'
           OR d.license_number ILIKE '%' || p_search || '%')
    -- By surname, the order the reader scans, so the page boundary follows what they see; d.id
    -- breaks ties. idx_drivers_company_name serves the company-filtered page without a sort.
    ORDER BY d.last_name ASC, d.first_name ASC, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT) IS
'One page of a tenant''s drivers by surname, optionally narrowed by a name or licence search, carrier and status; every row carries its carrier, its linked account''s email and the total_count the filters match';

CREATE FUNCTION tracking.sp_get_driver(
    p_tenant_id UUID,
    p_driver_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    company_id UUID,
    company_name VARCHAR,
    user_id UUID,
    user_email VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    phone VARCHAR,
    license_number VARCHAR,
    license_class VARCHAR,
    license_expires_on DATE,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

-- Refused while a route still names the driver as its default: the FK would set those routes'
-- default_driver_id to NULL, which is never what the click meant. The app names the routes back.
CREATE FUNCTION tracking.sp_delete_driver(
    p_tenant_id UUID,
    p_driver_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    v_deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.routes r WHERE r.default_driver_id = p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.has-routes' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.drivers d
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    RETURN v_deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_driver(UUID, UUID) IS
'Deletes a driver; raises tracking.driver.has-routes while a route still names them as its default driver';
