CREATE OR REPLACE FUNCTION tracking.sp_replace_calendar_dates(p_tenant_id uuid, p_calendar_id uuid, p_dates jsonb)
 RETURNS TABLE(date date, kind character varying, label character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_from DATE;
    v_to   DATE;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.calendars c
        WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'calendar.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT (e->>'date')::DATE AS d
        FROM jsonb_array_elements(COALESCE(p_dates, '[]'::jsonb)) e
        GROUP BY (e->>'date')::DATE
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'calendar.duplicate-date' USING ERRCODE = 'P0001';
    END IF;

    CREATE TEMP TABLE tmp_incoming_dates ON COMMIT DROP AS
    SELECT
        (e->>'date')::DATE                   AS date,
        CAST(e->>'kind' AS VARCHAR)          AS kind,
        CAST(NULLIF(TRIM(e->>'label'), '') AS VARCHAR) AS label
    FROM jsonb_array_elements(COALESCE(p_dates, '[]'::jsonb)) e;

    -- The symmetric difference of what is stored and what is arriving: the dates whose service
    -- actually changes. NULL labels compare equal under EXCEPT, which is what is wanted here.
    SELECT MIN(d.date), MAX(d.date) INTO v_from, v_to
    FROM (
        (SELECT i.date, i.kind, i.label FROM tmp_incoming_dates i
         EXCEPT
         SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
         FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id)
        UNION
        (SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
         FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id
         EXCEPT
         SELECT i.date, i.kind, i.label FROM tmp_incoming_dates i)
    ) d;

    DELETE FROM tracking.calendar_dates cd WHERE cd.calendar_id = p_calendar_id;

    INSERT INTO tracking.calendar_dates (calendar_id, date, kind, label)
    SELECT p_calendar_id, i.date, i.kind, i.label FROM tmp_incoming_dates i;

    -- Every route whose schedule reads this calendar now plans different days, over the span the
    -- change touched. One row per route, in this transaction (INFRA-001 D3).
    IF v_from IS NOT NULL THEN
        INSERT INTO tracking.outbox (topic, payload)
        SELECT DISTINCT
            'route.changed',
            jsonb_build_object(
                'route_id',  s.route_id,
                'date_from', v_from,
                'date_to',   v_to
            )
        FROM tracking.route_schedules s
        WHERE s.calendar_id = p_calendar_id;
    END IF;

    DROP TABLE tmp_incoming_dates;

    RETURN QUERY
    SELECT cd.date, CAST(cd.kind AS VARCHAR), CAST(cd.label AS VARCHAR)
    FROM tracking.calendar_dates cd
    WHERE cd.calendar_id = p_calendar_id
    ORDER BY cd.date ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_replace_calendar_dates(uuid,uuid,jsonb) IS 'Replaces a calendar''s whole date list in one statement and returns it; writes one route.changed row per affected route over the span of the dates that actually changed, and raises tracking.calendar.duplicate-date when the list names a date twice';
