-- TRACK-004 step 1: alerts become readable and resolvable, and a trip's event log readable.
--
-- Until now route_alerts was written by sp_create_route_alert (the office) and sp_driver_incident (the
-- driver's phone, TRACK-027) and read only by the active_alerts counters; nothing resolved a row, and
-- trip_events had no reader. Every raise and every resolve now also writes an alert.changed outbox row
-- in its own transaction, which the worker publishes as an `alert` frame (D1): zero cost at rest, one
-- publish per change.
--
-- route_alerts has no tenant column: an alert reaches its tenant through its route's company, as
-- sp_create_route_alert has always checked.

ALTER TABLE tracking.route_alerts
    ADD COLUMN resolved_by UUID;

-- The active slice is what the counters (route_id + status) and the page's default view read, and it
-- stays small however many resolved rows pile up.
CREATE INDEX idx_route_alerts_active ON tracking.route_alerts (route_id, created_at DESC) WHERE status = 'active';

-- ===========================================================================
-- Shared
-- ===========================================================================

-- One alert.changed outbox row { alert_id, route_id, trip_id, type: raised|resolved }.
CREATE FUNCTION tracking.fn_alert_changed(
    p_alert tracking.route_alerts,
    p_type VARCHAR
)
RETURNS VOID AS $$
BEGIN
    INSERT INTO tracking.outbox (topic, payload)
    VALUES ('alert.changed', jsonb_build_object(
        'alert_id', p_alert.id,
        'route_id', p_alert.route_id,
        'trip_id',  p_alert.trip_id,
        'type',     p_type
    ));
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_alert_changed(tracking.route_alerts, VARCHAR) IS
'Writes one alert.changed outbox row { alert_id, route_id, trip_id, type } for a raised or resolved alert (TRACK-004 D1)';

-- The one row shape of an alert: its columns plus the route, the plate and who raised and resolved it.
-- p_ids NULL is every alert of the tenant. The table's timestamps are without zone, written in the
-- session's; they are answered as instants, read in that same zone.
CREATE FUNCTION tracking.fn_route_alert_rows(
    p_tenant_id UUID,
    p_ids UUID[]
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    estimated_delay_minutes INT,
    status VARCHAR,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    created_by UUID,
    created_by_name VARCHAR,
    created_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID,
    resolved_by_name VARCHAR
) AS $$
    SELECT
        a.id,
        a.route_id,
        CAST(r.route_name AS VARCHAR),
        a.vehicle_id,
        CAST(v.plate_number AS VARCHAR),
        a.trip_id,
        CAST(a.alert_type AS VARCHAR),
        CAST(COALESCE(a.title, a.alert_type) AS VARCHAR),
        a.message,
        CAST(a.severity AS VARCHAR),
        a.estimated_delay_minutes,
        CAST(a.status AS VARCHAR),
        ST_Y(CAST(a.location AS GEOMETRY)),
        ST_X(CAST(a.location AS GEOMETRY)),
        a.created_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', cu.first_name, cu.last_name), ''), cu.email) AS VARCHAR),
        CAST(a.created_at AS TIMESTAMPTZ),
        CAST(a.resolved_at AS TIMESTAMPTZ),
        a.resolved_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', ru.first_name, ru.last_name), ''), ru.email) AS VARCHAR)
    FROM tracking.route_alerts a
    INNER JOIN tracking.routes r ON r.id = a.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    LEFT JOIN tracking.vehicles v ON v.id = a.vehicle_id
    LEFT JOIN auth.users cu ON cu.id = a.created_by AND cu.deleted_at IS NULL
    LEFT JOIN auth.users ru ON ru.id = a.resolved_by AND ru.deleted_at IS NULL
    WHERE tc.tenant_id = p_tenant_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids));
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_route_alert_rows(UUID, UUID[]) IS
'The row shape of the tenant''s alerts (all when p_ids is NULL): route_alerts columns plus route_name, license_plate, lat/lng and the raiser''s and resolver''s names (TRACK-004)';

-- ===========================================================================
-- List and resolve
-- ===========================================================================

-- Newest first. p_status and p_severity NULL are every one; the endpoint defaults p_status to active.
CREATE FUNCTION tracking.sp_list_route_alerts(
    p_tenant_id UUID,
    p_route_id UUID,
    p_status VARCHAR,
    p_severity VARCHAR,
    p_page INT,
    p_page_size INT
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    estimated_delay_minutes INT,
    status VARCHAR,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    created_by UUID,
    created_by_name VARCHAR,
    created_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID,
    resolved_by_name VARCHAR,
    total_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT a.id, a.route_id, a.route_name, a.vehicle_id, a.license_plate, a.trip_id, a.alert_type, a.title,
           a.message, a.severity, a.estimated_delay_minutes, a.status, a.lat, a.lng, a.created_by,
           a.created_by_name, a.created_at, a.resolved_at, a.resolved_by, a.resolved_by_name,
           CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.fn_route_alert_rows(p_tenant_id, NULL) a
    WHERE (p_route_id IS NULL OR a.route_id = p_route_id)
      AND (p_status IS NULL OR a.status = p_status)
      AND (p_severity IS NULL OR a.severity = p_severity)
    ORDER BY a.created_at DESC, a.id DESC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_route_alerts(UUID, UUID, VARCHAR, VARCHAR, INT, INT) IS
'One page of the tenant''s alerts, newest first, narrowed by route, status and severity when set, with total_count (TRACK-004)';

-- Resolves an active alert once. Another tenant's id is alert.not-found; a second resolve is
-- alert.already-resolved. The row is locked so two operators resolving at once get one success.
CREATE FUNCTION tracking.sp_resolve_route_alert(
    p_tenant_id UUID,
    p_alert_id UUID,
    p_user_id UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    route_name VARCHAR,
    vehicle_id UUID,
    license_plate VARCHAR,
    trip_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    estimated_delay_minutes INT,
    status VARCHAR,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    created_by UUID,
    created_by_name VARCHAR,
    created_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID,
    resolved_by_name VARCHAR
) AS $$
DECLARE
    v_alert tracking.route_alerts%ROWTYPE;
BEGIN
    SELECT a.* INTO v_alert
    FROM tracking.route_alerts a
    INNER JOIN tracking.routes r ON r.id = a.route_id
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE a.id = p_alert_id AND tc.tenant_id = p_tenant_id
    FOR UPDATE OF a;
    IF v_alert.id IS NULL THEN
        RAISE EXCEPTION 'alert.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_alert.status <> 'active' THEN
        RAISE EXCEPTION 'alert.already-resolved' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.route_alerts a
    SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, resolved_by = p_user_id, updated_at = CURRENT_TIMESTAMP
    WHERE a.id = v_alert.id
    RETURNING a.* INTO v_alert;

    PERFORM tracking.fn_alert_changed(v_alert, 'resolved');

    RETURN QUERY SELECT * FROM tracking.fn_route_alert_rows(p_tenant_id, ARRAY[v_alert.id]);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_resolve_route_alert(UUID, UUID, UUID) IS
'Resolves an active alert (status, resolved_at, resolved_by) and writes alert.changed resolved; raises alert.not-found, alert.already-resolved (TRACK-004)';

-- ===========================================================================
-- Raising writes alert.changed too — same signatures, bodies only
-- ===========================================================================

CREATE OR REPLACE FUNCTION tracking.sp_create_route_alert(
    p_tenant_id UUID,
    p_route_id UUID,
    p_vehicle_id UUID,
    p_alert_type VARCHAR(50),
    p_title VARCHAR(255),
    p_message TEXT,
    p_severity VARCHAR(50),
    p_estimated_delay_minutes INTEGER,
    p_created_by UUID
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    vehicle_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    estimated_delay_minutes INT,
    status VARCHAR,
    created_by UUID,
    created_at TIMESTAMP
) AS $$
DECLARE
    v_alert tracking.route_alerts%ROWTYPE;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, alert_type, title, message, severity, estimated_delay_minutes, created_by, status
    )
    VALUES (
        p_route_id,
        p_vehicle_id,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_title), ''), p_alert_type),
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        COALESCE(NULLIF(TRIM(p_severity), ''), 'medium'),
        p_estimated_delay_minutes,
        p_created_by,
        'active'
    )
    RETURNING * INTO v_alert;

    PERFORM tracking.fn_alert_changed(v_alert, 'raised');

    RETURN QUERY SELECT
        v_alert.id,
        v_alert.route_id,
        v_alert.vehicle_id,
        CAST(v_alert.alert_type AS VARCHAR),
        CAST(v_alert.title AS VARCHAR),
        v_alert.message,
        CAST(v_alert.severity AS VARCHAR),
        v_alert.estimated_delay_minutes,
        CAST(v_alert.status AS VARCHAR),
        v_alert.created_by,
        v_alert.created_at;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_create_route_alert(UUID, UUID, UUID, VARCHAR, VARCHAR, TEXT, VARCHAR, INTEGER, UUID) IS
'Raises an alert on a tenant''s route and writes alert.changed raised; raises route.not-found (TRACK-004)';

CREATE OR REPLACE FUNCTION tracking.sp_driver_incident(
    p_tenant_id UUID,
    p_user_id UUID,
    p_trip_id UUID,
    p_alert_type VARCHAR,
    p_message TEXT,
    p_lat DOUBLE PRECISION,
    p_lng DOUBLE PRECISION
)
RETURNS TABLE(
    id UUID,
    route_id UUID,
    vehicle_id UUID,
    trip_id UUID,
    alert_type VARCHAR,
    title VARCHAR,
    message TEXT,
    severity VARCHAR,
    status VARCHAR,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION,
    created_by UUID,
    created_at TIMESTAMP
) AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
    v_trip   tracking.trips%ROWTYPE;
    v_alert  tracking.route_alerts%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    SELECT t.* INTO v_trip
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id AND t.driver_id = v_driver.id;
    IF v_trip.id IS NULL THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_alerts (
        route_id, vehicle_id, trip_id, alert_type, title, message, severity, location, created_by, status
    )
    VALUES (
        v_trip.route_id,
        v_trip.vehicle_id,
        v_trip.id,
        p_alert_type,
        p_alert_type,
        COALESCE(NULLIF(TRIM(p_message), ''), ''),
        CASE p_alert_type WHEN 'emergency' THEN 'critical' WHEN 'breakdown' THEN 'high' ELSE 'medium' END,
        CAST(ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326) AS GEOGRAPHY),
        p_user_id,
        'active'
    )
    RETURNING * INTO v_alert;

    PERFORM tracking.fn_trip_changed(p_tenant_id, v_trip.id, 'incident', p_user_id, jsonb_build_object(
        'alert_id',   v_alert.id,
        'alert_type', v_alert.alert_type,
        'lat',        p_lat,
        'lng',        p_lng
    ));
    PERFORM tracking.fn_alert_changed(v_alert, 'raised');

    RETURN QUERY SELECT
        v_alert.id,
        v_alert.route_id,
        v_alert.vehicle_id,
        v_alert.trip_id,
        CAST(v_alert.alert_type AS VARCHAR),
        CAST(v_alert.title AS VARCHAR),
        v_alert.message,
        CAST(v_alert.severity AS VARCHAR),
        CAST(v_alert.status AS VARCHAR),
        p_lat,
        p_lng,
        v_alert.created_by,
        v_alert.created_at;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_driver_incident(UUID, UUID, UUID, VARCHAR, TEXT, DOUBLE PRECISION, DOUBLE PRECISION) IS
'The driver account reports an incident on one of their trips: a route_alerts row with trip_id and location, an incident trip_events row, a trip.changed and an alert.changed outbox row; raises trip.not-found, driver.not-linked (TRACK-027 D2, TRACK-004 D1)';

-- ===========================================================================
-- A trip's event log
-- ===========================================================================

-- Oldest first, as they happened. Another tenant's trip is trip.not-found.
CREATE FUNCTION tracking.sp_list_trip_events(
    p_tenant_id UUID,
    p_trip_id UUID
)
RETURNS TABLE(
    id UUID,
    type VARCHAR,
    payload JSONB,
    created_at TIMESTAMPTZ,
    created_by_name VARCHAR
) AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tracking.trips t WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        e.id,
        CAST(e.type AS VARCHAR),
        e.payload,
        e.created_at,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR)
    FROM tracking.trip_events e
    LEFT JOIN auth.users u ON u.id = e.created_by AND u.deleted_at IS NULL
    WHERE e.trip_id = p_trip_id
    ORDER BY e.created_at, e.id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_list_trip_events(UUID, UUID) IS
'A tenant''s trip''s events oldest first, each with who made it; raises trip.not-found (TRACK-004 D3)';
