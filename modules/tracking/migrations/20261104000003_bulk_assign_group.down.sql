-- TRACK-039 rollback: no group, no bulk add.

DROP FUNCTION tracking.sp_bulk_assign_riders(UUID, UUID, UUID[], SMALLINT, DATE);
DROP FUNCTION tracking.sp_list_rider_groups(UUID, UUID);
DROP FUNCTION tracking.sp_create_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, DECIMAL, DECIMAL, TEXT, VARCHAR);
DROP FUNCTION tracking.sp_update_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, BOOLEAN, UUID, DECIMAL, DECIMAL, TEXT, VARCHAR);
DROP FUNCTION tracking.sp_get_rider(UUID, UUID, UUID, UUID);
DROP FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT, UUID, UUID, UUID, BOOLEAN, VARCHAR);

CREATE FUNCTION tracking.sp_create_rider(
    p_tenant_id UUID,
    p_organization_id UUID,
    p_rider_type VARCHAR(50),
    p_first_name VARCHAR(100),
    p_last_name VARCHAR(100),
    p_identification_number VARCHAR(50),
    p_phone VARCHAR(20),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(20),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(20),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_scope_user_id UUID DEFAULT NULL,
    p_home_latitude DECIMAL DEFAULT NULL,
    p_home_longitude DECIMAL DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT
) AS $$
DECLARE
    v_rider_id UUID;
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.riders (
        organization_id, rider_type, first_name, last_name,
        identification_number, phone, email, status, home_location, notes
    )
    VALUES (
        p_organization_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        'active',
        CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography END,
        NULLIF(TRIM(p_notes), '')
    )
    RETURNING tracking.riders.id INTO v_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_update_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_phone VARCHAR(50),
    p_email VARCHAR(255),
    p_emergency_contact_name VARCHAR(255),
    p_emergency_contact_phone VARCHAR(50),
    p_guardian_user_id UUID,
    p_guardian_name VARCHAR(255),
    p_guardian_phone VARCHAR(50),
    p_guardian_email VARCHAR(255),
    p_address VARCHAR(500),
    p_is_active BOOLEAN,
    p_scope_user_id UUID DEFAULT NULL,
    p_home_latitude DECIMAL DEFAULT NULL,
    p_home_longitude DECIMAL DEFAULT NULL,
    p_notes TEXT DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT
) AS $$
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

    UPDATE tracking.riders r SET
        phone = NULLIF(TRIM(p_phone), ''),
        email = NULLIF(TRIM(p_email), ''),
        status = CASE
            WHEN p_is_active IS NOT NULL THEN CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END
            ELSE r.status
        END,
        home_location = CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography END,
        notes = NULLIF(TRIM(p_notes), ''),
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;

    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(p_address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_riders(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL,
    p_search VARCHAR DEFAULT NULL,
    p_rider_type VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20,
    p_scope_user_id UUID DEFAULT NULL,
    p_guardian_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    organization_name VARCHAR,
    organization_kind VARCHAR,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT,
    total_count BIGINT
) AS $$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(o.name AS VARCHAR),
        CAST(o.kind AS VARCHAR),
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope))
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_rider_type IS NULL OR p_rider_type = '' OR r.rider_type = p_rider_type)
      AND (p_is_active IS NULL OR (r.status = 'active') = p_is_active)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR r.first_name            ILIKE '%' || p_search || '%'
          OR r.last_name             ILIKE '%' || p_search || '%'
          OR r.identification_number ILIKE '%' || p_search || '%'
      )
    -- Read alphabetically, so the page is cut the same way the reader sees it; r.id breaks ties.
    ORDER BY r.last_name ASC, r.first_name ASC, r.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT, UUID, UUID) IS
'One page of a tenant''s riders by name with their organization''s name and kind, narrowed to p_scope_user_id''s organizations and to p_guardian_user_id''s own riders when either is set, and optionally by organization, type, status and a name/identification search; every row carries the total_count the filters match';

CREATE FUNCTION tracking.sp_get_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardian_user_id UUID,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    organization_id UUID,
    rider_type VARCHAR,
    first_name VARCHAR,
    last_name VARCHAR,
    identification_number VARCHAR,
    phone VARCHAR,
    email VARCHAR,
    emergency_contact_name VARCHAR,
    emergency_contact_phone VARCHAR,
    guardian_user_id UUID,
    guardian_name VARCHAR,
    guardian_phone VARCHAR,
    guardian_email VARCHAR,
    address VARCHAR,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    home_latitude DECIMAL,
    home_longitude DECIMAL,
    notes TEXT
) AS $$
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

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        NULL::VARCHAR,
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

DROP INDEX tracking.idx_riders_organization_group;
ALTER TABLE tracking.riders DROP COLUMN group_label;
