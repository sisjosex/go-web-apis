-- TRACK-001 step 1: the two fleet lists the app pages move to the purchasing shape —
-- p_page / p_page_size in, total_count out — so the endpoints answer
-- { <plural>, total_count, page, page_size } and the app pages server-side.
--
-- Signatures and return types change: DROP + CREATE (sql.md). p_page_size is capped by the
-- controller's binding (max 100); the SPs trust it.
--
-- Three changes beyond paging:
--   order      both lists are read alphabetically (TRACK-001 steps 3 and 4), so the page is cut
--              that way: a Table sorting its own page would only sort the rows on screen.
--   companies  p_registration_number becomes p_search, matching name OR registration number —
--              one box in the app's DataView instead of a field-specific filter.
--   vehicles   p_company_id was already nullable here; only the controller required it (D2).
--              company_name costs nothing new: the join to transport_companies is what scopes
--              the rows to the tenant, so the name is already in hand.
--
-- Cost: both lists are one query with COUNT(*) OVER () — neither builds anything per row that
-- LIMIT would throw away. The trigram indexes serve the '%term%' searches; the composite
-- indexes serve filter + order so the page is not a sort of the whole table.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ===========================================================================
-- Indexes
-- ===========================================================================
CREATE INDEX IF NOT EXISTS idx_transport_companies_tenant_name
    ON tracking.transport_companies (tenant_id, name, id);

CREATE INDEX IF NOT EXISTS idx_transport_companies_name_trgm
    ON tracking.transport_companies USING GIN (name gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_transport_companies_reg_trgm
    ON tracking.transport_companies USING GIN (registration_number gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_vehicles_company_plate
    ON tracking.vehicles (company_id, plate_number, id);

CREATE INDEX IF NOT EXISTS idx_vehicles_plate_trgm
    ON tracking.vehicles USING GIN (plate_number gin_trgm_ops);

-- ===========================================================================
-- sp_list_companies
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_list_companies(UUID, VARCHAR, VARCHAR);
CREATE FUNCTION tracking.sp_list_companies(
    p_tenant_id UUID,
    p_search    VARCHAR DEFAULT NULL,
    p_status    VARCHAR DEFAULT NULL,
    p_page      INT     DEFAULT 1,
    p_page_size INT     DEFAULT 20
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    city VARCHAR,
    country VARCHAR,
    registration_number VARCHAR,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    total_count BIGINT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT
        tc.id,
        tc.tenant_id,
        CAST(tc.name AS VARCHAR),
        CAST(tc.email AS VARCHAR),
        CAST(tc.phone AS VARCHAR),
        tc.address,
        CAST(tc.city AS VARCHAR),
        CAST(tc.country AS VARCHAR),
        CAST(tc.registration_number AS VARCHAR),
        CAST(tc.status AS VARCHAR),
        tc.created_at,
        tc.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.transport_companies tc
    WHERE tc.tenant_id = p_tenant_id
      AND (
          p_search IS NULL
          OR p_search = ''
          OR tc.name                ILIKE '%' || p_search || '%'
          OR tc.registration_number ILIKE '%' || p_search || '%'
      )
      AND (p_status IS NULL OR p_status = '' OR tc.status = p_status)
    -- The list is read alphabetically, so the page is cut that way too: ordering by creation
    -- date and sorting the page in the browser would only sort the twenty rows on screen.
    -- tc.id breaks ties, so two companies of the same name never swap between pages.
    ORDER BY tc.name ASC, tc.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$;

COMMENT ON FUNCTION tracking.sp_list_companies(UUID, VARCHAR, VARCHAR, INT, INT) IS
'One page of a tenant''s transport companies by name, optionally narrowed by a name/registration-number search and a status; every row carries the total_count the filters match';

-- ===========================================================================
-- sp_list_vehicles
-- ===========================================================================
DROP FUNCTION IF EXISTS tracking.sp_list_vehicles(UUID, UUID, VARCHAR, VARCHAR);
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
'One page of a tenant''s vehicles by plate number, optionally narrowed by company, type, status and a plate search; every row carries its company name and the total_count the filters match. p_company_id NULL lists the whole fleet (TRACK-001 D2)';
