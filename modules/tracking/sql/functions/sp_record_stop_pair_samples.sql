CREATE OR REPLACE FUNCTION tracking.sp_record_stop_pair_samples(p_tenant_id uuid, p_trip_id uuid)
 RETURNS integer
 LANGUAGE sql
AS $function$
    -- Consecutive arrived stops of a completed trip, a skipped stop bridged: arrival to arrival, read in
    -- the trip's own zone for the weekday and the 2-hour band. A replay finds its rows and adds none.
    WITH arrived AS (
        SELECT ts.stop_place_id, ts.arrived_at, t.timezone,
            LEAD(ts.stop_place_id) OVER w AS next_stop_place_id,
            LEAD(ts.arrived_at) OVER w AS next_arrived_at
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trips t ON t.id = ts.trip_id
        WHERE t.id = p_trip_id
          AND t.tenant_id = p_tenant_id
          AND t.status = 'completed'
          AND ts.arrived_at IS NOT NULL
        WINDOW w AS (ORDER BY ts.sequence)
    ),
    inserted AS (
        INSERT INTO tracking.stop_pair_samples
            (trip_id, from_stop_place_id, to_stop_place_id, tenant_id, weekday, hour_band, duration_s, observed_at)
        SELECT p_trip_id, a.stop_place_id, a.next_stop_place_id, p_tenant_id,
            CAST(EXTRACT(ISODOW FROM a.arrived_at AT TIME ZONE a.timezone) AS SMALLINT),
            CAST(EXTRACT(HOUR FROM a.arrived_at AT TIME ZONE a.timezone) AS SMALLINT) / 2,
            CAST(EXTRACT(EPOCH FROM a.next_arrived_at - a.arrived_at) AS INT),
            a.arrived_at
        FROM arrived a
        WHERE a.next_stop_place_id IS NOT NULL
          AND a.next_stop_place_id <> a.stop_place_id
          AND a.next_arrived_at > a.arrived_at + INTERVAL '1 second'
        ON CONFLICT DO NOTHING
        RETURNING 1
    )
    SELECT CAST(COUNT(*) AS INT) FROM inserted;
$function$;

COMMENT ON FUNCTION tracking.sp_record_stop_pair_samples(uuid,uuid) IS 'Records a completed trip''s arrival-to-arrival seconds per pair of consecutive arrived stops; answers the rows added (TRACK-066 D1)';
