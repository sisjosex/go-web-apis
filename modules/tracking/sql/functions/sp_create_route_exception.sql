CREATE OR REPLACE FUNCTION tracking.sp_create_route_exception(p_tenant_id uuid, p_route_id uuid, p_date_from date, p_date_to date, p_kind character varying, p_payload jsonb, p_reason character varying, p_created_by uuid)
 RETURNS TABLE(id uuid, route_id uuid, date_from date, date_to date, kind character varying, payload jsonb, reason character varying, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF NOT tracking.fn_exception_payload_valid(p_kind, p_payload) THEN
        RAISE EXCEPTION 'exception.payload' USING ERRCODE = 'P0001';
    END IF;

    -- An id the payload names has to be this tenant's, or the preview — and TRACK-008 after it —
    -- would hand a reader another tenant's bus.
    IF p_kind = 'change_vehicle' AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = (p_payload->>'vehicle_id')::UUID AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'vehicle.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_kind = 'change_driver' AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
        WHERE d.id = (p_payload->>'driver_id')::UUID AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_kind IN ('skip_stop', 'add_stop') AND NOT EXISTS (
        SELECT 1 FROM tracking.stop_places sp
        WHERE sp.id = (p_payload->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- A detour names at least two of the tenant's stop places (TRACK-065 D3).
    IF p_kind = 'detour' AND (jsonb_array_length(p_payload->'stops') < 2 OR EXISTS (
        SELECT 1 FROM jsonb_array_elements_text(p_payload->'stops') s(id)
        WHERE NOT EXISTS (
            SELECT 1 FROM tracking.stop_places sp
            WHERE sp.id = CAST(s.id AS UUID) AND sp.tenant_id = p_tenant_id
        )
    )) THEN
        RAISE EXCEPTION 'exception.payload' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.route_exceptions (route_id, date_from, date_to, kind, payload, reason, created_by)
    VALUES (
        p_route_id, p_date_from, p_date_to, p_kind,
        COALESCE(p_payload, '{}'::jsonb), NULLIF(TRIM(p_reason), ''), p_created_by
    )
    RETURNING tracking.route_exceptions.id INTO v_id;

    PERFORM tracking.fn_route_changed(p_route_id, p_date_from, p_date_to);

    RETURN QUERY
    SELECT
        e.id, e.route_id, e.date_from, e.date_to, CAST(e.kind AS VARCHAR), e.payload,
        CAST(e.reason AS VARCHAR), e.created_by, e.created_at, e.updated_at
    FROM tracking.route_exceptions e
    WHERE e.id = v_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_route_exception(uuid,uuid,date,date,character varying,jsonb,character varying,uuid) IS 'Records what happens differently over a range of dates; raises tracking.exception.payload when the payload does not carry exactly the keys its kind defines, and writes one route.changed row over that range';
