CREATE OR REPLACE FUNCTION tracking.sp_roll_stop_delay_stats(p_tenant_id uuid, p_weeks integer)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    -- Signed minutes: an early stop pulls the median down, as it should for an expectation.
    WITH samples AS (
        SELECT t.route_id, ts.stop_place_id,
               CAST(EXTRACT(ISODOW FROM t.service_date) AS SMALLINT) AS weekday,
               EXTRACT(EPOCH FROM (ts.arrived_at - ts.planned_at)) / 60 AS delay_min
        FROM tracking.trips t
        INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
        WHERE t.tenant_id = p_tenant_id
          AND t.status = 'completed'
          AND t.service_date >= CURRENT_DATE - p_weeks * 7
          AND ts.arrived_at IS NOT NULL
          AND ts.planned_at IS NOT NULL
    ),
    stats AS (
        SELECT sm.route_id, sm.stop_place_id, sm.weekday, CAST(COUNT(*) AS INT) AS samples,
               ROUND(CAST(percentile_cont(0.5) WITHIN GROUP (ORDER BY sm.delay_min) AS NUMERIC), 1) AS p50,
               ROUND(CAST(percentile_cont(0.9) WITHIN GROUP (ORDER BY sm.delay_min) AS NUMERIC), 1) AS p90
        FROM samples sm
        GROUP BY sm.route_id, sm.stop_place_id, sm.weekday
    ),
    stale AS (
        DELETE FROM tracking.stop_delay_stats d
        WHERE d.tenant_id = p_tenant_id
          AND NOT EXISTS (
              SELECT 1 FROM stats x
              WHERE x.route_id = d.route_id AND x.stop_place_id = d.stop_place_id AND x.weekday = d.weekday
          )
    ),
    upserted AS (
        INSERT INTO tracking.stop_delay_stats AS d (
            route_id, stop_place_id, weekday, tenant_id, samples, p50_delay_min, p90_delay_min, computed_at
        )
        SELECT x.route_id, x.stop_place_id, x.weekday, p_tenant_id, x.samples, x.p50, x.p90, now()
        FROM stats x
        ON CONFLICT (route_id, stop_place_id, weekday) DO UPDATE SET
            samples = EXCLUDED.samples,
            p50_delay_min = EXCLUDED.p50_delay_min,
            p90_delay_min = EXCLUDED.p90_delay_min,
            computed_at = EXCLUDED.computed_at
        RETURNING 1
    )
    SELECT CAST(COUNT(*) AS INT) INTO v_count FROM upserted;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_roll_stop_delay_stats(uuid,integer) IS 'Recomputes the tenant''s p50/p90 minutes past planned_at per route, stop and ISO weekday over the completed trips of the last p_weeks, dropping combinations no longer seen; answers the rows written (TRACK-013 D5)';
