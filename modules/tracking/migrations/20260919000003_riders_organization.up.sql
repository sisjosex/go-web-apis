-- TRACK-005 step 1 (D1): a rider belongs to the school or employer that sends them, not to the
-- carrier that drives them. `riders.organization_id` replaces `company_id`; the carrier is reached
-- through the route the rider is assigned to, which is where it always belonged.
--
-- Backfill: every carrier that has riders becomes one `kind='other'` organization carrying the
-- carrier's own id, so `organization_id := company_id` is the whole repoint and the down migration
-- can read it back. The operator renames it and sets its real kind afterwards.
--
-- Every SP that scoped a rider by joining `transport_companies` is redefined here: that join was the
-- tenant scope, and it no longer exists. Signatures change, so DROP + CREATE (sql.md).
-- sp_list_riders also takes its final paged shape (purchasing envelope) in the same pass, rather
-- than being rewritten twice inside one spec.

ALTER TABLE tracking.riders
    ADD COLUMN organization_id UUID REFERENCES tracking.organizations(id) ON DELETE CASCADE;

INSERT INTO tracking.organizations (id, tenant_id, kind, name, timezone, is_active)
SELECT tc.id, tc.tenant_id, 'other', tc.name, 'UTC', true
FROM tracking.transport_companies tc
WHERE EXISTS (SELECT 1 FROM tracking.riders r WHERE r.company_id = tc.id);

UPDATE tracking.riders r SET organization_id = r.company_id;

ALTER TABLE tracking.riders ALTER COLUMN organization_id SET NOT NULL;

-- Dropping the column takes its FK and idx_riders_company_id with it.
ALTER TABLE tracking.riders DROP COLUMN company_id;

-- Filter + order in one index, so a page is not a sort of the whole table; the trigram indexes serve
-- the '%term%' search over the two name columns.
CREATE INDEX idx_riders_org_name ON tracking.riders (organization_id, last_name, first_name, id);
CREATE INDEX idx_riders_first_name_trgm ON tracking.riders USING GIN (first_name gin_trgm_ops);
CREATE INDEX idx_riders_last_name_trgm ON tracking.riders USING GIN (last_name gin_trgm_ops);

DROP FUNCTION IF EXISTS tracking.sp_create_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_update_rider(UUID, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, VARCHAR, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_list_riders(UUID, UUID, UUID, VARCHAR, BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_get_rider(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_delete_rider(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_get_rider_status(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_assign_rider(UUID, UUID, UUID, UUID, UUID, VARCHAR);
DROP FUNCTION IF EXISTS tracking.sp_unassign_rider(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_assignments(UUID, UUID, UUID, BOOLEAN);

-- ===========================================================================
-- sp_create_rider
-- ===========================================================================
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
    p_address VARCHAR(500)
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
    updated_at TIMESTAMP
) AS $$
DECLARE
    v_rider_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.riders (
        organization_id, rider_type, first_name, last_name,
        identification_number, phone, email, status
    )
    VALUES (
        p_organization_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        'active'
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
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = v_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_update_rider
-- ===========================================================================
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
    p_is_active BOOLEAN
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
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
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
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_list_riders — one page, purchasing envelope
-- ===========================================================================
-- p_organization_id NULL lists every rider the tenant has; the search matches either name or the
-- identification number, one box in the app's DataView instead of three field-specific filters.
CREATE FUNCTION tracking.sp_list_riders(
    p_tenant_id UUID,
    p_organization_id UUID DEFAULT NULL,
    p_search VARCHAR DEFAULT NULL,
    p_rider_type VARCHAR DEFAULT NULL,
    p_is_active BOOLEAN DEFAULT NULL,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
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
    total_count BIGINT
) AS $$
BEGIN
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
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.riders r
    -- Not a LEFT JOIN: organization_id is NOT NULL and this join is what scopes the rows to the
    -- tenant, so an outer join would only widen the result to other tenants' riders.
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE o.tenant_id = p_tenant_id
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

COMMENT ON FUNCTION tracking.sp_list_riders(UUID, UUID, VARCHAR, VARCHAR, BOOLEAN, INT, INT) IS
'One page of a tenant''s riders by name, optionally narrowed by organization, type, status and a name/identification search; every row carries the total_count the filters match';

-- ===========================================================================
-- sp_get_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_get_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_guardian_user_id UUID
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
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
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
        r.updated_at
    FROM tracking.riders r
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_delete_rider
-- ===========================================================================
CREATE FUNCTION tracking.sp_delete_rider(
    p_tenant_id UUID,
    p_rider_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.riders r WHERE r.id = p_rider_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_get_rider_status — body unchanged, only the tenant guard moves to organizations
-- ===========================================================================
CREATE FUNCTION tracking.sp_get_rider_status(
    p_tenant_id UUID,
    p_rider_id UUID
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    driver_name VARCHAR,
    vehicle_latitude DECIMAL,
    vehicle_longitude DECIMAL,
    vehicle_speed DECIMAL,
    location_age_seconds INT,
    last_event_type VARCHAR,
    last_event_time TIMESTAMP,
    last_event_notes TEXT,
    last_event_stop VARCHAR,
    scheduled_pickup_stop VARCHAR,
    scheduled_dropoff_stop VARCHAR,
    active_alerts INT
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        rider.id AS rider_id,
        CAST(rider.first_name || ' ' || rider.last_name AS VARCHAR) AS rider_name,
        route.id AS route_id,
        CAST(route.route_name AS VARCHAR),
        v.id AS vehicle_id,
        CAST(v.plate_number AS VARCHAR) AS license_plate,
        NULL::VARCHAR AS driver_name,
        vl.latitude AS vehicle_latitude,
        vl.longitude AS vehicle_longitude,
        vl.speed AS vehicle_speed,
        CAST(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - vl.recorded_at)) AS INT) AS location_age_seconds,
        CAST(last_event.event_type AS VARCHAR) AS last_event_type,
        last_event.event_time AS last_event_time,
        last_event.notes AS last_event_notes,
        CAST(last_event.stop_name AS VARCHAR) AS last_event_stop,
        CAST(pickup_stop.location_name AS VARCHAR) AS scheduled_pickup_stop,
        CAST(dropoff_stop.location_name AS VARCHAR) AS scheduled_dropoff_stop,
        COALESCE((
            SELECT COUNT(*)::INT
            FROM tracking.route_alerts alerts
            WHERE alerts.route_id = ra.route_id
              AND alerts.status = 'active'
        ), 0) AS active_alerts
    FROM tracking.riders rider
    LEFT JOIN LATERAL (
        SELECT ra_inner.route_id, ra_inner.pickup_stop_id, ra_inner.dropoff_stop_id
        FROM tracking.rider_assignments ra_inner
        INNER JOIN tracking.routes ro ON ro.id = ra_inner.route_id
        WHERE ra_inner.rider_id = rider.id
          AND ra_inner.status = 'active'
          AND ro.scheduled_end_time >= LOCALTIME
        ORDER BY ro.scheduled_start_time, ro.id
        LIMIT 1
    ) ra ON true
    LEFT JOIN tracking.routes route ON route.id = ra.route_id
    LEFT JOIN tracking.vehicles v ON v.id = route.vehicle_id
    LEFT JOIN LATERAL (
        SELECT vl_inner.latitude, vl_inner.longitude, vl_inner.speed, vl_inner.recorded_at
        FROM tracking.vehicle_locations vl_inner
        WHERE vl_inner.vehicle_id = v.id
        ORDER BY vl_inner.recorded_at DESC
        LIMIT 1
    ) vl ON true
    LEFT JOIN LATERAL (
        SELECT event_inner.event_type, event_inner.event_time, event_inner.notes, event_stop.location_name AS stop_name
        FROM tracking.ride_events event_inner
        LEFT JOIN tracking.route_stops event_stop ON event_stop.id = event_inner.stop_id
        WHERE event_inner.rider_id = p_rider_id
        ORDER BY event_inner.event_time DESC, event_inner.created_at DESC
        LIMIT 1
    ) last_event ON true
    LEFT JOIN tracking.route_stops pickup_stop ON pickup_stop.id = ra.pickup_stop_id
    LEFT JOIN tracking.route_stops dropoff_stop ON dropoff_stop.id = ra.dropoff_stop_id
    WHERE rider.id = p_rider_id;
END;
$$ LANGUAGE plpgsql;

-- ===========================================================================
-- sp_assign_rider / sp_unassign_rider / sp_list_rider_assignments
-- ===========================================================================
-- The rider is scoped through its organization, the route still through its carrier: an assignment
-- is exactly where the two sides meet.
CREATE FUNCTION tracking.sp_assign_rider(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_pickup_stop_id UUID,
    p_dropoff_stop_id UUID,
    p_status VARCHAR(50)
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.rider_assignments ra
        WHERE ra.rider_id = p_rider_id AND ra.route_id = p_route_id AND ra.status = 'active'
    ) THEN
        RAISE EXCEPTION 'assignment.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.rider_assignments (
        rider_id, route_id, pickup_stop_id, dropoff_stop_id, status
    )
    VALUES (
        p_rider_id,
        p_route_id,
        p_pickup_stop_id,
        p_dropoff_stop_id,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.rider_assignments.id,
        tracking.rider_assignments.rider_id,
        tracking.rider_assignments.route_id,
        tracking.rider_assignments.pickup_stop_id,
        tracking.rider_assignments.dropoff_stop_id,
        CAST(tracking.rider_assignments.status AS VARCHAR),
        tracking.rider_assignments.created_at,
        tracking.rider_assignments.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_unassign_rider(
    p_tenant_id UUID,
    p_assignment_id UUID
)
RETURNS BOOLEAN AS $$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.rider_assignments ra
        INNER JOIN tracking.riders r ON r.id = ra.rider_id
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE ra.id = p_assignment_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'assignment.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.rider_assignments ra WHERE ra.id = p_assignment_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION tracking.sp_list_rider_assignments(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_route_id UUID,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    rider_id UUID,
    route_id UUID,
    pickup_stop_id UUID,
    dropoff_stop_id UUID,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        ra.id,
        ra.rider_id,
        ra.route_id,
        ra.pickup_stop_id,
        ra.dropoff_stop_id,
        CAST(ra.status AS VARCHAR),
        ra.created_at,
        ra.updated_at
    FROM tracking.rider_assignments ra
    INNER JOIN tracking.riders r ON r.id = ra.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE o.tenant_id = p_tenant_id
      AND (p_rider_id IS NULL OR ra.rider_id = p_rider_id)
      AND (p_route_id IS NULL OR ra.route_id = p_route_id)
      AND (p_is_active IS NULL OR (ra.status = 'active') = p_is_active)
    ORDER BY ra.created_at DESC;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_list_rider_assignments(UUID, UUID, UUID, BOOLEAN) IS
'A tenant''s rider-route assignments, optionally narrowed by rider, route and status; the rider is scoped through its organization, the route through its carrier';
