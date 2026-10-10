CREATE OR REPLACE FUNCTION tracking.fn_route_version_stops_json(p_tenant_id uuid, p_version_id uuid, p_stops jsonb)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_item     RECORD;
    v_place    UUID;
    v_point    GEOGRAPHY;
    v_resolved JSONB := '[]'::jsonb;
    v_inserted INT;
BEGIN
    FOR v_item IN
        SELECT e.value, e.ordinality
        FROM jsonb_array_elements(COALESCE(p_stops, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality)
    LOOP
        IF NULLIF(v_item.value->>'stop_place_id', '') IS NOT NULL THEN
            SELECT sp.id INTO v_place
            FROM tracking.stop_places sp
            WHERE sp.id = (v_item.value->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id;
            IF v_place IS NULL THEN
                RAISE EXCEPTION 'stop-place.not-found' USING ERRCODE = 'P0001';
            END IF;
        ELSE
            IF NULLIF(TRIM(v_item.value->>'name'), '') IS NULL
               OR v_item.value->>'latitude' IS NULL
               OR v_item.value->>'longitude' IS NULL THEN
                RAISE EXCEPTION 'stop-place.invalid' USING ERRCODE = 'P0001';
            END IF;
            v_point := ST_SetSRID(ST_MakePoint(
                (v_item.value->>'longitude')::DOUBLE PRECISION,
                (v_item.value->>'latitude')::DOUBLE PRECISION), 4326)::geography;

            -- The nearest place of the tenant within 30 m is the same stop under another click.
            SELECT sp.id INTO v_place
            FROM tracking.stop_places sp
            WHERE sp.tenant_id = p_tenant_id
              AND sp.location IS NOT NULL
              AND ST_DWithin(sp.location, v_point, 30)
            ORDER BY ST_Distance(sp.location, v_point)
            LIMIT 1;

            IF v_place IS NULL THEN
                INSERT INTO tracking.stop_places (tenant_id, name, address, location)
                VALUES (p_tenant_id, TRIM(v_item.value->>'name'), NULLIF(TRIM(v_item.value->>'address'), ''), v_point)
                RETURNING tracking.stop_places.id INTO v_place;
            END IF;
        END IF;

        v_resolved := v_resolved || jsonb_build_array(
            v_item.value || jsonb_build_object('stop_place_id', v_place, 'ord', v_item.ordinality));
    END LOOP;

    INSERT INTO tracking.route_version_stops (version_id, stop_place_id, sequence, planned_offset_min, dwell_sec)
    SELECT
        p_version_id,
        (e->>'stop_place_id')::UUID,
        CAST(ROW_NUMBER() OVER (ORDER BY COALESCE((e->>'sequence')::INT, (e->>'ord')::INT), (e->>'ord')::INT) AS INT),
        (e->>'planned_offset_min')::INT,
        (e->>'dwell_sec')::INT
    FROM jsonb_array_elements(v_resolved) e;
    GET DIAGNOSTICS v_inserted = ROW_COUNT;
    RETURN v_inserted;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_version_stops_json(uuid,uuid,jsonb) IS 'Writes an editor''s stop list onto a version, renumbering it 1..n in the order the caller asked for; an item without stop_place_id is a new place {name, address, latitude, longitude}, reusing the tenant''s place within 30 m (TRACK-034 D3). Raises tracking.stop-place.not-found or tracking.stop-place.invalid';
