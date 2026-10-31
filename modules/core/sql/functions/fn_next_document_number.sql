CREATE OR REPLACE FUNCTION core.fn_next_document_number(p_tenant_id uuid, p_series text, p_prefix text, p_pad integer DEFAULT 6, p_yearly boolean DEFAULT false)
 RETURNS text
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_year  INT := 0;
    v_value BIGINT;
BEGIN
    IF p_yearly
    THEN
        v_year := EXTRACT(YEAR FROM now() AT TIME ZONE 'America/La_Paz');
    END IF;

    -- One statement holds the row lock until the caller's transaction ends: two documents of one series
    -- number one after the other, and a rollback returns the number.
    INSERT INTO core.document_counters AS dc (tenant_id, series, year, next_value)
    VALUES (p_tenant_id, p_series, v_year, 1)
    ON CONFLICT (tenant_id, series, year) DO UPDATE SET next_value = dc.next_value + 1
    RETURNING dc.next_value INTO v_value;

    IF p_yearly
    THEN
        RETURN p_prefix || '-' || v_year || '-' || lpad(v_value::TEXT, p_pad, '0');
    END IF;

    RETURN p_prefix || '-' || lpad(v_value::TEXT, p_pad, '0');
END;
$function$;

COMMENT ON FUNCTION core.fn_next_document_number(uuid,text,text,integer,boolean) IS 'The next gapless number of a tenant''s series, as PREFIX-000123, or PREFIX-2026-000123 when it restarts each Bolivian year. Call it inside the document''s transaction (CORE-001 D1).';
