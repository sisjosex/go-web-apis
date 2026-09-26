-- TRACK-027 step 1: the rider card's QR and the driver's incident report.
--
-- A rider carries an opaque token (D1): 20 random bytes in base32, printed on their card. It is no
-- secret and no PII — scanning it only answers the rider and their pending pickup on the scanning
-- driver's trip — and rotating it voids that one card. Riders have no tenant column (the tenant is
-- reached through their organization), so the token is unique across the table: 160 random bits never
-- collide, and one index probe resolves a scan.
--
-- An incident is a route_alerts row that also names its trip and where the driver was, plus an
-- `incident` trip_events row and a trip.changed outbox row through fn_trip_changed, so the board hears
-- of it on the WebSocket as of any other change of the trip (D2).

-- 20 random bytes as 32 base32 characters (RFC 4648 alphabet, no padding): five bytes at a time are
-- 40 bits, eight characters.
CREATE FUNCTION tracking.fn_new_qr_token()
RETURNS TEXT AS $$
DECLARE
    c_alphabet CONSTANT TEXT := 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    v_bytes    BYTEA := gen_random_bytes(20);
    v_bits     BIGINT;
    v_token    TEXT := '';
BEGIN
    FOR g IN 0..3 LOOP
        v_bits := 0;
        FOR i IN 0..4 LOOP
            v_bits := (v_bits << 8) | get_byte(v_bytes, g * 5 + i);
        END LOOP;
        FOR j IN REVERSE 7..0 LOOP
            v_token := v_token || substr(c_alphabet, ((v_bits >> (j * 5)) & 31)::INT + 1, 1);
        END LOOP;
    END LOOP;
    RETURN v_token;
END;
$$ LANGUAGE plpgsql VOLATILE;

COMMENT ON FUNCTION tracking.fn_new_qr_token() IS
'A new rider card token: 20 random bytes as 32 base32 characters (TRACK-027 D1)';

-- The volatile default gives every existing rider its own token.
ALTER TABLE tracking.riders
    ADD COLUMN qr_token TEXT NOT NULL DEFAULT tracking.fn_new_qr_token(),
    ADD CONSTRAINT uk_riders_qr_token UNIQUE (qr_token);

ALTER TABLE tracking.route_alerts
    ADD COLUMN trip_id UUID REFERENCES tracking.trips(id) ON DELETE SET NULL,
    ADD COLUMN location GEOGRAPHY(Point, 4326);

-- The FK's own index: a planned trip the materialiser drops would otherwise scan every alert.
CREATE INDEX idx_route_alerts_trip_id ON tracking.route_alerts (trip_id) WHERE trip_id IS NOT NULL;

-- ===========================================================================
-- The card's token
-- ===========================================================================

-- Reads a rider's token, or replaces it when p_rotate. An organization user reaches only the riders
-- of their organizations; out of scope reads as rider.not-found (TRACK-015 D1).
CREATE FUNCTION tracking.sp_rider_qr_token(
    p_tenant_id UUID,
    p_rider_id UUID,
    p_rotate BOOLEAN,
    p_scope_user_id UUID DEFAULT NULL
)
RETURNS TEXT AS $$
DECLARE
    v_scope UUID[];
    v_token TEXT;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_rotate THEN
        UPDATE tracking.riders r
        SET qr_token = tracking.fn_new_qr_token(), updated_at = CURRENT_TIMESTAMP
        FROM tracking.organizations o
        WHERE o.id = r.organization_id
          AND r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope))
        RETURNING r.qr_token INTO v_token;
    ELSE
        SELECT r.qr_token INTO v_token
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope));
    END IF;

    IF v_token IS NULL THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_token;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_rider_qr_token(UUID, UUID, BOOLEAN, UUID) IS
'A rider''s card token, replaced first when p_rotate (the old one then resolves nothing); raises rider.not-found, also out of an organization user''s scope (TRACK-027 D1)';

-- ===========================================================================
-- The driver finds a rider
-- ===========================================================================

-- A rider's pending pickup on the driver's trips in progress, the earliest stop first; no row when
-- there is none.
CREATE FUNCTION tracking.fn_driver_pending_pickup(
    p_tenant_id UUID,
    p_driver_id UUID,
    p_rider_id UUID
)
RETURNS TABLE(
    task_id UUID,
    trip_id UUID,
    trip_stop_id UUID,
    kind VARCHAR,
    status VARCHAR
) AS $$
    SELECT k.id, t.id, ts.id, CAST(k.kind AS VARCHAR), CAST(k.status AS VARCHAR)
    FROM tracking.trips t
    INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
    INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
    WHERE t.tenant_id = p_tenant_id
      AND t.driver_id = p_driver_id
      AND t.status = 'in_progress'
      AND k.subject_type = 'passenger'
      AND k.subject_id = p_rider_id
      AND k.kind = 'pickup'
      AND k.status = 'pending'
    ORDER BY t.started_at, ts.sequence
    LIMIT 1;
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.fn_driver_pending_pickup(UUID, UUID, UUID) IS
'A rider''s pending pickup on a driver''s trips in progress, earliest stop first (TRACK-027)';

-- Resolves a scanned (or typed) card: the tenant's rider holding the token and their pending pickup
-- on the driver's trip in progress, task columns NULL when there is none. The token is compared
-- uppercased and without blanks, so a code typed by hand matches. Unknown, rotated or another tenant's
-- token → rider.not-found.
CREATE FUNCTION tracking.sp_driver_resolve_rider(
    p_tenant_id UUID,
    p_user_id UUID,
    p_token TEXT
)
RETURNS TABLE(
    rider_id UUID,
    rider_name TEXT,
    task_id UUID,
    trip_id UUID,
    trip_stop_id UUID,
    kind VARCHAR,
    status VARCHAR
) AS $$
DECLARE
    v_driver tracking.drivers%ROWTYPE;
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT rd.id, rd.first_name || ' ' || rd.last_name, pk.task_id, pk.trip_id, pk.trip_stop_id, pk.kind, pk.status
    FROM tracking.riders rd
    INNER JOIN tracking.organizations o ON o.id = rd.organization_id
    LEFT JOIN LATERAL tracking.fn_driver_pending_pickup(p_tenant_id, v_driver.id, rd.id) pk ON true
    WHERE rd.qr_token = upper(regexp_replace(p_token, '\s', '', 'g'))
      AND o.tenant_id = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_driver_resolve_rider(UUID, UUID, TEXT) IS
'The rider a card token names and their pending pickup on the driver account''s trip in progress (task columns NULL when none); raises rider.not-found, driver.not-linked (TRACK-027 D1)';

-- The driver's search when a card cannot be scanned (D1): the riders on the driver's trips in
-- progress whose name contains p_query, or whose card code is p_query. Only those riders — a driver
-- never browses the tenant's riders — at most 20, by name.
CREATE FUNCTION tracking.sp_driver_find_riders(
    p_tenant_id UUID,
    p_user_id UUID,
    p_query TEXT
)
RETURNS TABLE(
    rider_id UUID,
    rider_name TEXT,
    task_id UUID,
    trip_id UUID,
    trip_stop_id UUID,
    kind VARCHAR,
    status VARCHAR
) AS $$
DECLARE
    v_driver  tracking.drivers%ROWTYPE;
    v_pattern TEXT := '%' || replace(replace(replace(btrim(p_query), '\', '\\'), '%', '\%'), '_', '\_') || '%';
    v_code    TEXT := upper(regexp_replace(p_query, '\s', '', 'g'));
BEGIN
    v_driver := tracking.fn_driver_of_user(p_tenant_id, p_user_id);

    RETURN QUERY
    SELECT rd.id, rd.first_name || ' ' || rd.last_name, pk.task_id, pk.trip_id, pk.trip_stop_id, pk.kind, pk.status
    FROM tracking.riders rd
    LEFT JOIN LATERAL tracking.fn_driver_pending_pickup(p_tenant_id, v_driver.id, rd.id) pk ON true
    WHERE rd.id IN (
        SELECT k.subject_id
        FROM tracking.trips t
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
        WHERE t.tenant_id = p_tenant_id
          AND t.driver_id = v_driver.id
          AND t.status = 'in_progress'
          AND k.subject_type = 'passenger'
    )
      AND ((rd.first_name || ' ' || rd.last_name) ILIKE v_pattern OR rd.qr_token = v_code)
    ORDER BY rd.last_name, rd.first_name, rd.id
    LIMIT 20;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_driver_find_riders(UUID, UUID, TEXT) IS
'Riders on the driver account''s trips in progress whose name contains p_query or whose card code is p_query, each with their pending pickup; at most 20; raises driver.not-linked (TRACK-027 D1)';

-- ===========================================================================
-- The driver reports an incident
-- ===========================================================================

-- An alert on the trip's route and vehicle, as sp_create_route_alert writes one, plus the trip and
-- where the driver stood; severity follows the type. Another driver's trip is trip.not-found (TRACK-011
-- D4), whatever its status: a bus may break down before it starts.
CREATE FUNCTION tracking.sp_driver_incident(
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
'The driver account reports an incident on one of their trips: a route_alerts row with trip_id and location, an incident trip_events row and a trip.changed outbox row; raises trip.not-found, driver.not-linked (TRACK-027 D2)';
