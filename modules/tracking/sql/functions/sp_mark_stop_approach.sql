CREATE OR REPLACE FUNCTION tracking.sp_mark_stop_approach(p_tenant_id uuid, p_trip_id uuid, p_trip_stop_ids uuid[])
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_stop  UUID;
    v_count INT := 0;
BEGIN
    -- The column is the once-per-stop guarantee shared with the radius in sp_ingest_positions: whichever
    -- reaches a stop first announces it.
    FOR v_stop IN
        UPDATE tracking.trip_stops ts
        SET approach_notified_at = now()
        FROM tracking.trips t
        WHERE t.id = p_trip_id
          AND t.tenant_id = p_tenant_id
          AND t.status = 'in_progress'
          AND ts.trip_id = t.id
          AND ts.id = ANY(p_trip_stop_ids)
          AND ts.status = 'pending'
          AND ts.approach_notified_at IS NULL
        RETURNING ts.id
    LOOP
        PERFORM tracking.fn_trip_changed(p_tenant_id, p_trip_id, 'approach', NULL,
            jsonb_build_object('trip_stop_id', v_stop));
        v_count := v_count + 1;
    END LOOP;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_mark_stop_approach(uuid,uuid,uuid[]) IS 'Announces the approach of the given pending stops of a running trip whose ETA is within the notice minutes (TRACK-070 D3): stamps approach_notified_at where still empty and writes one trip.changed approach per stop stamped, as the radius does (TRACK-012 D4); answers how many it stamped';
