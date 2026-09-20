-- TRACK-016 step 1: the documents slice — the tenant's policy (document_types), the papers filed
-- against a vehicle or a driver (compliance_documents), and the one probe that says whether an
-- expired paper stops them working (D2).
--
-- `status` is computed here, not in Go and not in the browser: "expiring" means "inside this type's
-- own warn_days_before", which only the row's join to its type knows. A client that worked it out
-- itself would need the policy shipped to it and would drift the day the policy changes.
--
-- Every document read joins its type and its subject, so a row renders without a second call — the
-- same shape the drivers and vehicles lists already answer with.

-- ===========================================================================
-- fn_service_blocked (D2)
-- ===========================================================================
-- LANGUAGE sql, not plpgsql: this runs once per row of the drivers and vehicles lists, and a SQL
-- function is inlined into that query, where a plpgsql one would be a separate call per row. The
-- predicate is exactly the leading columns of idx_documents_subject, so each probe is one index
-- lookup and no extra round-trip leaves Go.
CREATE FUNCTION tracking.fn_service_blocked(
    p_tenant_id UUID,
    p_subject_type VARCHAR,
    p_subject_id UUID
)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
    SELECT EXISTS (
        SELECT 1
        FROM tracking.compliance_documents d
        INNER JOIN tracking.document_types t ON t.id = d.document_type_id
        WHERE d.tenant_id = p_tenant_id
          AND d.subject_type = p_subject_type
          AND d.subject_id = p_subject_id
          AND t.blocks_service
          AND t.is_active
          AND d.expires_on IS NOT NULL
          AND d.expires_on < CURRENT_DATE
    );
$$;

COMMENT ON FUNCTION tracking.fn_service_blocked(UUID, VARCHAR, UUID) IS
'Whether an expired document of a blocks_service type stands against this vehicle or driver (TRACK-016 D2); the lists show it and TRACK-008''s assignment refuses on it';

-- ===========================================================================
-- Document types — the tenant's policy
-- ===========================================================================
CREATE FUNCTION tracking.sp_create_document_type(
    p_tenant_id UUID,
    p_code VARCHAR(50),
    p_name VARCHAR(255),
    p_applies_to VARCHAR(20),
    p_warn_days_before INT,
    p_blocks_service BOOLEAN,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    code VARCHAR,
    name VARCHAR,
    applies_to VARCHAR,
    warn_days_before INT,
    blocks_service BOOLEAN,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.tenant_id = p_tenant_id AND t.code = UPPER(TRIM(p_code))
    ) THEN
        RAISE EXCEPTION 'document-type.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.document_types (
        tenant_id, code, name, applies_to, warn_days_before, blocks_service, is_active
    )
    VALUES (
        p_tenant_id,
        UPPER(TRIM(p_code)),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_applies_to), ''), 'both'),
        COALESCE(p_warn_days_before, 30),
        COALESCE(p_blocks_service, false),
        COALESCE(p_is_active, true)
    )
    RETURNING
        tracking.document_types.id,
        tracking.document_types.tenant_id,
        CAST(tracking.document_types.code AS VARCHAR),
        CAST(tracking.document_types.name AS VARCHAR),
        CAST(tracking.document_types.applies_to AS VARCHAR),
        tracking.document_types.warn_days_before,
        tracking.document_types.blocks_service,
        tracking.document_types.is_active,
        tracking.document_types.created_at,
        tracking.document_types.updated_at;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_document_type(UUID, VARCHAR, VARCHAR, VARCHAR, INT, BOOLEAN, BOOLEAN) IS
'Creates a document type for one tenant; the code is upper-cased and unique per tenant, raising tracking.document-type.code-already-exists';

-- A NULL argument keeps the stored value, so a PATCH sending one field changes one field. Nothing
-- here is immutable: the policy is meant to be edited, and every document references the type by id.
CREATE FUNCTION tracking.sp_update_document_type(
    p_tenant_id UUID,
    p_document_type_id UUID,
    p_code VARCHAR(50),
    p_name VARCHAR(255),
    p_applies_to VARCHAR(20),
    p_warn_days_before INT,
    p_blocks_service BOOLEAN,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    code VARCHAR,
    name VARCHAR,
    applies_to VARCHAR,
    warn_days_before INT,
    blocks_service BOOLEAN,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document-type.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_code IS NOT NULL AND TRIM(p_code) <> '' AND EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.tenant_id = p_tenant_id
          AND t.code = UPPER(TRIM(p_code))
          AND t.id <> p_document_type_id
    ) THEN
        RAISE EXCEPTION 'document-type.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.document_types t SET
        code             = COALESCE(NULLIF(UPPER(TRIM(p_code)), ''), t.code),
        name             = COALESCE(NULLIF(TRIM(p_name), ''), t.name),
        applies_to       = COALESCE(NULLIF(TRIM(p_applies_to), ''), t.applies_to),
        warn_days_before = COALESCE(p_warn_days_before, t.warn_days_before),
        blocks_service   = COALESCE(p_blocks_service, t.blocks_service),
        is_active        = COALESCE(p_is_active, t.is_active),
        updated_at       = CURRENT_TIMESTAMP
    WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id
    RETURNING
        t.id,
        t.tenant_id,
        CAST(t.code AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(t.applies_to AS VARCHAR),
        t.warn_days_before,
        t.blocks_service,
        t.is_active,
        t.created_at,
        t.updated_at;
END;
$$ LANGUAGE plpgsql;

-- The whole set, not a page: a tenant's policy is a handful of rows that the document modal needs in
-- one go to offer the types a subject may carry.
CREATE FUNCTION tracking.sp_list_document_types(
    p_tenant_id  UUID,
    p_applies_to VARCHAR DEFAULT NULL,
    p_is_active  BOOLEAN DEFAULT NULL
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    code VARCHAR,
    name VARCHAR,
    applies_to VARCHAR,
    warn_days_before INT,
    blocks_service BOOLEAN,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.tenant_id,
        CAST(t.code AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(t.applies_to AS VARCHAR),
        t.warn_days_before,
        t.blocks_service,
        t.is_active,
        t.created_at,
        t.updated_at
    FROM tracking.document_types t
    WHERE t.tenant_id = p_tenant_id
      -- 'both' answers either subject, so asking for vehicle types returns the vehicle ones and it.
      AND (p_applies_to IS NULL OR p_applies_to = '' OR t.applies_to IN (p_applies_to, 'both'))
      AND (p_is_active IS NULL OR t.is_active = p_is_active)
    ORDER BY t.name ASC, t.id ASC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_document_types(UUID, VARCHAR, BOOLEAN) IS
'A tenant''s whole document policy alphabetically; p_applies_to narrows to the types one subject kind may carry, which includes the both-kinds ones';

-- ===========================================================================
-- Compliance documents
-- ===========================================================================
-- Resolves the subject's own name, so a document row reads as "Insurance · ABC-123" without the
-- caller knowing which table subject_id points at.
CREATE FUNCTION tracking.fn_document_subject_name(
    p_tenant_id UUID,
    p_subject_type VARCHAR,
    p_subject_id UUID
)
RETURNS VARCHAR
LANGUAGE sql
STABLE
AS $$
    SELECT CASE p_subject_type
        WHEN 'driver' THEN (
            SELECT CAST(d.first_name || ' ' || d.last_name AS VARCHAR)
            FROM tracking.drivers d
            WHERE d.id = p_subject_id AND d.tenant_id = p_tenant_id
        )
        WHEN 'vehicle' THEN (
            SELECT CAST(v.plate_number AS VARCHAR)
            FROM tracking.vehicles v
            INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
            WHERE v.id = p_subject_id AND tc.tenant_id = p_tenant_id
        )
    END;
$$;

CREATE FUNCTION tracking.sp_create_document(
    p_tenant_id UUID,
    p_subject_type VARCHAR(20),
    p_subject_id UUID,
    p_document_type_id UUID,
    p_number VARCHAR(100),
    p_issued_on DATE,
    p_expires_on DATE,
    p_notes TEXT
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    subject_type VARCHAR,
    subject_id UUID,
    subject_name VARCHAR,
    document_type_id UUID,
    type_name VARCHAR,
    number VARCHAR,
    issued_on DATE,
    expires_on DATE,
    file_name VARCHAR,
    file_size BIGINT,
    file_ext VARCHAR,
    notes TEXT,
    status VARCHAR,
    warned_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_applies_to VARCHAR;
    v_document_id UUID;
BEGIN
    SELECT t.applies_to INTO v_applies_to
    FROM tracking.document_types t
    WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id;

    IF v_applies_to IS NULL THEN
        RAISE EXCEPTION 'document-type.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_applies_to NOT IN (p_subject_type, 'both') THEN
        RAISE EXCEPTION 'document.type-not-applicable' USING ERRCODE = 'P0001';
    END IF;

    -- No FK reaches the subject (subject_type decides the table), so this is the check that keeps a
    -- document from being filed against a driver or vehicle that is not this tenant's, or not there.
    IF tracking.fn_document_subject_name(p_tenant_id, p_subject_type, p_subject_id) IS NULL THEN
        RAISE EXCEPTION 'document.subject-not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.compliance_documents (
        tenant_id, subject_type, subject_id, document_type_id, number, issued_on, expires_on, notes
    )
    VALUES (
        p_tenant_id,
        p_subject_type,
        p_subject_id,
        p_document_type_id,
        NULLIF(TRIM(p_number), ''),
        p_issued_on,
        p_expires_on,
        NULLIF(TRIM(p_notes), '')
    )
    RETURNING tracking.compliance_documents.id INTO v_document_id;

    -- The single read is the one projection every document response shares; naming the columns
    -- keeps the scan order identical to it without a second copy of the CASE.
    RETURN QUERY
    SELECT
        g.id,
        g.tenant_id,
        g.subject_type,
        g.subject_id,
        g.subject_name,
        g.document_type_id,
        g.type_name,
        g.number,
        g.issued_on,
        g.expires_on,
        g.file_name,
        g.file_size,
        g.file_ext,
        g.notes,
        g.status,
        g.warned_at,
        g.created_at,
        g.updated_at
    FROM tracking.sp_get_document(p_tenant_id, v_document_id) g;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_document(UUID, VARCHAR, UUID, UUID, VARCHAR, DATE, DATE, TEXT) IS
'Files a document against one vehicle or driver; raises tracking.document-type.not-found, tracking.document.type-not-applicable or tracking.document.subject-not-found';

-- A NULL argument keeps the stored value. Changing expires_on clears warned_at, so a renewed document
-- is warned about again on its own schedule instead of staying silent forever (D1).
CREATE FUNCTION tracking.sp_update_document(
    p_tenant_id UUID,
    p_document_id UUID,
    p_number VARCHAR(100),
    p_issued_on DATE,
    p_expires_on DATE,
    p_notes TEXT
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    subject_type VARCHAR,
    subject_id UUID,
    subject_name VARCHAR,
    document_type_id UUID,
    type_name VARCHAR,
    number VARCHAR,
    issued_on DATE,
    expires_on DATE,
    file_name VARCHAR,
    file_size BIGINT,
    file_ext VARCHAR,
    notes TEXT,
    status VARCHAR,
    warned_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.compliance_documents d
        WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.compliance_documents d SET
        number     = COALESCE(NULLIF(TRIM(p_number), ''), d.number),
        issued_on  = COALESCE(p_issued_on, d.issued_on),
        expires_on = COALESCE(p_expires_on, d.expires_on),
        notes      = COALESCE(NULLIF(TRIM(p_notes), ''), d.notes),
        warned_at  = CASE
                         WHEN p_expires_on IS NOT NULL AND p_expires_on IS DISTINCT FROM d.expires_on
                         THEN NULL
                         ELSE d.warned_at
                     END,
        updated_at = CURRENT_TIMESTAMP
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id;

    -- The single read is the one projection every document response shares; naming the columns
    -- keeps the scan order identical to it without a second copy of the CASE.
    RETURN QUERY
    SELECT
        g.id,
        g.tenant_id,
        g.subject_type,
        g.subject_id,
        g.subject_name,
        g.document_type_id,
        g.type_name,
        g.number,
        g.issued_on,
        g.expires_on,
        g.file_name,
        g.file_size,
        g.file_ext,
        g.notes,
        g.status,
        g.warned_at,
        g.created_at,
        g.updated_at
    FROM tracking.sp_get_document(p_tenant_id, p_document_id) g;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_get_document(
    p_tenant_id UUID,
    p_document_id UUID
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    subject_type VARCHAR,
    subject_id UUID,
    subject_name VARCHAR,
    document_type_id UUID,
    type_name VARCHAR,
    number VARCHAR,
    issued_on DATE,
    expires_on DATE,
    file_name VARCHAR,
    file_size BIGINT,
    file_ext VARCHAR,
    notes TEXT,
    status VARCHAR,
    warned_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.compliance_documents d
        WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        CAST(d.subject_type AS VARCHAR),
        d.subject_id,
        tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id),
        d.document_type_id,
        CAST(t.name AS VARCHAR),
        CAST(d.number AS VARCHAR),
        d.issued_on,
        d.expires_on,
        CAST(d.file_name AS VARCHAR),
        d.file_size,
        CAST(d.file_ext AS VARCHAR),
        d.notes,
        CAST(CASE
            WHEN d.expires_on IS NULL THEN 'valid'
            WHEN d.expires_on < CURRENT_DATE THEN 'expired'
            WHEN d.expires_on <= CURRENT_DATE + t.warn_days_before THEN 'expiring'
            ELSE 'valid'
        END AS VARCHAR),
        d.warned_at,
        d.created_at,
        d.updated_at
    FROM tracking.compliance_documents d
    INNER JOIN tracking.document_types t ON t.id = d.document_type_id
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_documents(
    p_tenant_id             UUID,
    p_subject_type          VARCHAR DEFAULT NULL,
    p_subject_id            UUID    DEFAULT NULL,
    p_expiring_within_days  INT     DEFAULT NULL,
    p_page                  INT     DEFAULT 1,
    p_page_size             INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    subject_type VARCHAR,
    subject_id UUID,
    subject_name VARCHAR,
    document_type_id UUID,
    type_name VARCHAR,
    number VARCHAR,
    issued_on DATE,
    expires_on DATE,
    file_name VARCHAR,
    file_size BIGINT,
    file_ext VARCHAR,
    notes TEXT,
    status VARCHAR,
    warned_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        CAST(d.subject_type AS VARCHAR),
        d.subject_id,
        tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id),
        d.document_type_id,
        CAST(t.name AS VARCHAR),
        CAST(d.number AS VARCHAR),
        d.issued_on,
        d.expires_on,
        CAST(d.file_name AS VARCHAR),
        d.file_size,
        CAST(d.file_ext AS VARCHAR),
        d.notes,
        CAST(CASE
            WHEN d.expires_on IS NULL THEN 'valid'
            WHEN d.expires_on < CURRENT_DATE THEN 'expired'
            WHEN d.expires_on <= CURRENT_DATE + t.warn_days_before THEN 'expiring'
            ELSE 'valid'
        END AS VARCHAR),
        d.warned_at,
        d.created_at,
        d.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.compliance_documents d
    INNER JOIN tracking.document_types t ON t.id = d.document_type_id
    WHERE d.tenant_id = p_tenant_id
      AND (p_subject_type IS NULL OR p_subject_type = '' OR d.subject_type = p_subject_type)
      AND (p_subject_id IS NULL OR d.subject_id = p_subject_id)
      -- A document with no expiry never expires, so it is not part of "expiring within N days".
      AND (p_expiring_within_days IS NULL
           OR (d.expires_on IS NOT NULL AND d.expires_on <= CURRENT_DATE + p_expiring_within_days))
    -- Soonest first, which is the order the reader acts in; one with no expiry has nothing to act on
    -- and goes last. d.id breaks ties.
    ORDER BY d.expires_on ASC NULLS LAST, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_documents(UUID, VARCHAR, UUID, INT, INT, INT) IS
'One page of a tenant''s compliance documents by expiry, optionally narrowed to one subject or to what expires within N days; every row carries its type, its subject''s name, a status of valid/expiring/expired and the total_count the filters match';

-- Deleting the row leaves its file behind; the Go handler removes that, since the filesystem is not
-- the database's to touch.
CREATE FUNCTION tracking.sp_delete_document(
    p_tenant_id UUID,
    p_document_id UUID
)
RETURNS TABLE(
    file_ext VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.compliance_documents d
        WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    DELETE FROM tracking.compliance_documents d
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    RETURNING CAST(d.file_ext AS VARCHAR);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_delete_document(UUID, UUID) IS
'Deletes a document and returns the extension of the file it had, so the handler can remove that file; raises tracking.document.not-found';

-- ===========================================================================
-- The fleet lists gain service_blocked (D2)
-- ===========================================================================
-- Both gain a return column, so DROP + CREATE (sql.md). The probe is one index lookup per row on an
-- already-paged result, so a page costs page_size lookups and no extra round-trip.
DROP FUNCTION IF EXISTS tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT);
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
    service_blocked BOOLEAN,
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
        tracking.fn_service_blocked(d.tenant_id, 'driver', d.id),
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
    ORDER BY d.last_name ASC, d.first_name ASC, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_drivers(UUID, VARCHAR, UUID, VARCHAR, INT, INT) IS
'One page of a tenant''s drivers by surname, optionally narrowed by a name or licence search, carrier and status; every row carries its carrier, its linked account''s email, whether an expired blocking document stands against it (TRACK-016 D2) and the total_count the filters match';

DROP FUNCTION IF EXISTS tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT);
CREATE FUNCTION tracking.sp_list_vehicles(
    p_tenant_id    UUID,
    p_company_id   UUID    DEFAULT NULL,
    p_vehicle_type VARCHAR DEFAULT NULL,
    p_status       VARCHAR DEFAULT NULL,
    p_search       VARCHAR DEFAULT NULL,
    p_page         INT     DEFAULT 1,
    p_page_size    INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    company_id UUID,
    company_name VARCHAR,
    plate_number VARCHAR,
    vehicle_type VARCHAR,
    brand VARCHAR,
    model VARCHAR,
    year INT,
    capacity INT,
    gps_device_id VARCHAR,
    status VARCHAR,
    service_blocked BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        v.id,
        v.company_id,
        CAST(tc.name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        CAST(v.vehicle_type AS VARCHAR),
        CAST(v.brand AS VARCHAR),
        CAST(v.model AS VARCHAR),
        v.year,
        v.capacity,
        CAST(v.gps_device_id AS VARCHAR),
        CAST(v.status AS VARCHAR),
        tracking.fn_service_blocked(tc.tenant_id, 'vehicle', v.id),
        v.created_at,
        v.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.vehicles v
    -- Not a LEFT JOIN: company_id is NOT NULL and this join is what scopes the rows to the
    -- tenant, so an outer join would only widen the result to other tenants' vehicles.
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
    WHERE tc.tenant_id = p_tenant_id
      AND (p_company_id IS NULL OR v.company_id = p_company_id)
      AND (p_vehicle_type IS NULL OR p_vehicle_type = '' OR v.vehicle_type = p_vehicle_type)
      AND (p_status IS NULL OR p_status = '' OR v.status = p_status)
      AND (
          p_search IS NULL
          OR p_search = ''
          OR v.plate_number ILIKE '%' || p_search || '%'
      )
    -- Alphabetical by plate, for the same reason as the company list above.
    ORDER BY v.plate_number ASC, v.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, INT, INT) IS
'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status and a plate search; every row carries its company name, whether an expired blocking document stands against it (TRACK-016 D2) and the total_count the filters match. p_company_id NULL lists the whole fleet (TRACK-001 D2)';
