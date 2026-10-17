CREATE OR REPLACE FUNCTION tracking.fn_exception_payload_valid(p_kind character varying, p_payload jsonb)
 RETURNS boolean
 LANGUAGE plpgsql
 IMMUTABLE
AS $function$
DECLARE
    v_keys     TEXT[];
    v_expected TEXT[];
BEGIN
    IF jsonb_typeof(COALESCE(p_payload, '{}'::jsonb)) <> 'object' THEN
        RETURN FALSE;
    END IF;

    v_keys := ARRAY(SELECT k FROM jsonb_object_keys(COALESCE(p_payload, '{}'::jsonb)) k ORDER BY k);
    v_expected := CASE p_kind
        WHEN 'cancel'         THEN ARRAY[]::TEXT[]
        WHEN 'change_time'    THEN ARRAY['start_time']
        WHEN 'change_vehicle' THEN ARRAY['vehicle_id']
        WHEN 'change_driver'  THEN ARRAY['driver_id']
        WHEN 'skip_stop'      THEN ARRAY['stop_place_id']
        WHEN 'add_stop'       THEN ARRAY['after_sequence', 'stop_place_id']
        WHEN 'detour'         THEN ARRAY['stops']
    END;

    IF v_keys IS DISTINCT FROM v_expected THEN
        RETURN FALSE;
    END IF;

    IF p_kind = 'change_time' AND p_payload->>'start_time' !~ '^([01][0-9]|2[0-3]):[0-5][0-9]$' THEN
        RETURN FALSE;
    END IF;

    IF p_kind IN ('skip_stop', 'add_stop') AND p_payload->>'stop_place_id' !~* '^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$' THEN
        RETURN FALSE;
    END IF;

    IF p_kind = 'add_stop' AND (jsonb_typeof(p_payload->'after_sequence') <> 'number'
                                OR p_payload->>'after_sequence' !~ '^[0-9]+$') THEN
        RETURN FALSE;
    END IF;

    -- TRACK-065 D3: the day's whole list, in order, as stop place ids.
    IF p_kind = 'detour' AND jsonb_typeof(p_payload->'stops') <> 'array' THEN
        RETURN FALSE;
    END IF;

    IF p_kind = 'detour' AND EXISTS (
        SELECT 1 FROM jsonb_array_elements(p_payload->'stops') s
        WHERE jsonb_typeof(s) <> 'string' OR s #>> '{}' !~* '^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$'
    ) THEN
        RETURN FALSE;
    END IF;

    RETURN TRUE;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_exception_payload_valid(character varying,jsonb) IS 'Whether an exception payload carries exactly the keys its kind defines, and whether the values the preview and the day''s stop plan cast are castable: a uuid stop_place_id, a whole after_sequence, detour stops as a list of uuids (TRACK-018 step 3, TRACK-065)';
