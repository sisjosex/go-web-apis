CREATE OR REPLACE FUNCTION tracking.sp_add_scope_calendar_dates(p_tenant_id uuid, p_organization_id uuid, p_from date, p_to date, p_kind character varying, p_label character varying)
 RETURNS TABLE(calendar_id uuid, added integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope       VARCHAR := CASE WHEN p_organization_id IS NULL THEN 'tenant' ELSE 'organization' END;
    v_name        VARCHAR;
    v_calendar_id UUID;
    v_added       INT;
BEGIN
    IF p_to < p_from OR (p_to - p_from) > 91 THEN
        RAISE EXCEPTION 'calendar.range-too-wide' USING ERRCODE = 'P0001';
    END IF;

    IF p_organization_id IS NULL THEN
        v_name := 'Todas las rutas';
    ELSE
        SELECT o.name INTO v_name
        FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
        IF v_name IS NULL THEN
            RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    -- The scope's one calendar, made by its first date (TRACK-076 D1); the partial unique indexes
    -- settle two first dates written at once.
    SELECT c.id INTO v_calendar_id
    FROM tracking.calendars c
    WHERE c.tenant_id = p_tenant_id
      AND c.scope = v_scope
      AND (v_scope = 'tenant' OR c.organization_id = p_organization_id);
    IF v_calendar_id IS NULL THEN
        BEGIN
            INSERT INTO tracking.calendars (tenant_id, organization_id, name, scope)
            VALUES (p_tenant_id, p_organization_id, v_name, v_scope)
            RETURNING tracking.calendars.id INTO v_calendar_id;
        EXCEPTION WHEN unique_violation THEN
            SELECT c.id INTO v_calendar_id
            FROM tracking.calendars c
            WHERE c.tenant_id = p_tenant_id
              AND c.scope = v_scope
              AND (v_scope = 'tenant' OR c.organization_id = p_organization_id);
        END;
    END IF;

    INSERT INTO tracking.calendar_dates (calendar_id, date, kind, label)
    SELECT v_calendar_id, CAST(d AS DATE), p_kind, NULLIF(TRIM(p_label), '')
    FROM generate_series(p_from, p_to, INTERVAL '1 day') d
    ON CONFLICT ON CONSTRAINT pk_calendar_dates DO UPDATE SET
        kind  = EXCLUDED.kind,
        label = EXCLUDED.label;
    GET DIAGNOSTICS v_added = ROW_COUNT;

    PERFORM tracking.fn_route_changed(cr.route_id, p_from, p_to)
    FROM tracking.fn_calendar_routes(p_tenant_id, v_calendar_id) cr;

    RETURN QUERY SELECT v_calendar_id, v_added;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_add_scope_calendar_dates(uuid,uuid,date,date,character varying,character varying) IS 'Marks every day of a window of at most 92 days on the tenant calendar (p_organization_id NULL) or a client''s, creating that calendar on its first date; writes route.changed for every route the scope reaches; raises tracking.calendar.range-too-wide, tracking.organization.not-found (TRACK-076 D1)';
