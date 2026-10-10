CREATE OR REPLACE FUNCTION tracking.fn_organization_stop_sync(p_tenant_id uuid, p_organization_id uuid, p_create boolean)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_org   RECORD;
    v_stop  RECORD;
    v_place UUID;
BEGIN
    SELECT o.name, o.address, o.location, o.stop_place_id INTO v_org
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF v_org.location IS NULL THEN
        RETURN v_org.stop_place_id;
    END IF;

    IF v_org.stop_place_id IS NOT NULL THEN
        SELECT sp.name, sp.address, sp.location INTO v_stop
        FROM tracking.stop_places sp WHERE sp.id = v_org.stop_place_id;
        IF v_stop.location IS DISTINCT FROM v_org.location
            OR v_stop.name IS DISTINCT FROM v_org.name
            OR (v_org.address IS NOT NULL AND v_stop.address IS DISTINCT FROM v_org.address) THEN
            PERFORM tracking.sp_update_stop_place(p_tenant_id, v_org.stop_place_id, v_org.name, v_org.address,
                ST_Y(v_org.location::geometry), ST_X(v_org.location::geometry), p_organization_id);
        END IF;
        RETURN v_org.stop_place_id;
    END IF;

    IF NOT COALESCE(p_create, true) THEN
        RETURN NULL;
    END IF;

    SELECT sp.id INTO v_place
    FROM tracking.stop_places sp
    WHERE sp.tenant_id = p_tenant_id
      AND sp.location IS NOT NULL
      AND sp.organization_id = p_organization_id
      AND ST_DWithin(sp.location, v_org.location, 30)
    ORDER BY ST_Distance(sp.location, v_org.location)
    LIMIT 1;
    IF v_place IS NULL THEN
        INSERT INTO tracking.stop_places (tenant_id, organization_id, name, address, location)
        VALUES (p_tenant_id, p_organization_id, v_org.name, v_org.address, v_org.location)
        RETURNING tracking.stop_places.id INTO v_place;
    END IF;

    UPDATE tracking.organizations o SET stop_place_id = v_place WHERE o.id = p_organization_id;
    RETURN v_place;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_organization_stop_sync(uuid,uuid,boolean) IS 'Keeps the client''s stop on its pin and name (TRACK-045 D3); without one and p_create, links its nearest own stop within 30 m or a new one; NULL for a client without a pin';
