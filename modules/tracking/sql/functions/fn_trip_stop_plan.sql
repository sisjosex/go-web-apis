CREATE OR REPLACE FUNCTION tracking.fn_trip_stop_plan(p_tenant_id uuid, p_route_id uuid, p_version_id uuid, p_service_date date)
 RETURNS TABLE(stop_place_id uuid, sequence integer, planned_offset_min integer)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_places  UUID[];
    v_offsets INT[];
    v_stops   UUID[];
    v_e       RECORD;
    v_n       INT;
    v_at      INT;
    v_place   UUID;
    v_offset  INT;
    v_shift   INT;
BEGIN
    SELECT COALESCE(array_agg(vs.stop_place_id ORDER BY vs.sequence), '{}'),
           COALESCE(array_agg(vs.planned_offset_min ORDER BY vs.sequence), '{}')
      INTO v_places, v_offsets
    FROM tracking.route_version_stops vs
    WHERE vs.version_id = p_version_id;

    -- TRACK-065 D3: the day's exceptions over the version's list, oldest first, so a later one reads
    -- the list the earlier ones left.
    FOR v_e IN
        SELECT e.kind, e.payload
        FROM tracking.route_exceptions e
        WHERE e.route_id = p_route_id
          AND p_service_date BETWEEN e.date_from AND e.date_to
          AND e.kind IN ('skip_stop', 'add_stop', 'detour')
        ORDER BY e.created_at, e.id
    LOOP
        v_n := cardinality(v_places);

        IF v_e.kind = 'skip_stop' THEN
            v_place := (v_e.payload->>'stop_place_id')::UUID;
            SELECT COALESCE(array_agg(x.place ORDER BY x.i), '{}'), COALESCE(array_agg(x.off ORDER BY x.i), '{}')
              INTO v_places, v_offsets
            FROM unnest(v_places, v_offsets) WITH ORDINALITY AS x(place, off, i)
            WHERE x.place <> v_place;

        ELSIF v_e.kind = 'add_stop' THEN
            -- After stop v_at of the list as it stands; 0 puts it first, past the end appends it.
            v_place := (v_e.payload->>'stop_place_id')::UUID;
            v_at := LEAST(GREATEST((v_e.payload->>'after_sequence')::INT, 0), v_n);
            IF v_at = 0 THEN
                v_offset := COALESCE(v_offsets[1], 0);
                v_shift := CASE WHEN v_n > 0 THEN tracking.fn_stop_place_minutes(p_tenant_id, v_place, v_places[1]) ELSE 0 END;
            ELSE
                v_shift := tracking.fn_stop_place_minutes(p_tenant_id, v_places[v_at], v_place);
                v_offset := v_offsets[v_at] + v_shift;
            END IF;
            v_places := v_places[1:v_at] || v_place || v_places[v_at + 1:v_n];
            v_offsets := v_offsets[1:v_at] || v_offset
                      || ARRAY(SELECT x.off + v_shift FROM unnest(v_offsets[v_at + 1:v_n]) WITH ORDINALITY AS x(off, i) ORDER BY x.i);

        ELSE
            -- A detour is the whole list of the day, timed from the first stop's offset.
            v_stops := ARRAY(
                SELECT CAST(s.value #>> '{}' AS UUID)
                FROM jsonb_array_elements(v_e.payload->'stops') WITH ORDINALITY AS s(value, i)
                ORDER BY s.i);
            v_offset := COALESCE(v_offsets[1], 0);
            v_offsets := ARRAY[]::INT[];
            FOR v_at IN 1..cardinality(v_stops) LOOP
                IF v_at > 1 THEN
                    v_offset := v_offset + tracking.fn_stop_place_minutes(p_tenant_id, v_stops[v_at - 1], v_stops[v_at]);
                END IF;
                v_offsets := v_offsets || v_offset;
            END LOOP;
            v_places := v_stops;
        END IF;
    END LOOP;

    RETURN QUERY
    SELECT x.place, CAST(x.i AS INT), x.off
    FROM unnest(v_places, v_offsets) WITH ORDINALITY AS x(place, off, i);
END;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_stop_plan(uuid,uuid,uuid,date) IS 'The stops a route calls at on one date: the version''s list with that date''s skip_stop, add_stop and detour exceptions applied in creation order, renumbered 1..n; an added stop is timed by the TRACK-038 straight-line rule and pushes the later stops back by the same (TRACK-065 D3)';
