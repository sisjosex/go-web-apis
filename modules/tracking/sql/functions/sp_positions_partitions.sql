CREATE OR REPLACE FUNCTION tracking.sp_positions_partitions(p_ahead_days integer, p_retention_days integer DEFAULT NULL::integer)
 RETURNS TABLE(created integer, dropped integer)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_today   DATE := CAST(now() AT TIME ZONE 'UTC' AS DATE);
    v_day     DATE;
    v_name    TEXT;
    v_created INT := 0;
    v_dropped INT := 0;
    v_cutoff  DATE;
    v_part    RECORD;
BEGIN
    FOR v_day IN SELECT CAST(d AS DATE) FROM generate_series(v_today, v_today + p_ahead_days, INTERVAL '1 day') d LOOP
        v_name := 'vehicle_positions_p' || to_char(v_day, 'YYYYMMDD');
        CONTINUE WHEN to_regclass('tracking.' || v_name) IS NOT NULL;

        -- Rows of this day that fell into the default partition must leave it before the day's
        -- partition can exist.
        IF to_regclass('pg_temp.tmp_positions_moved') IS NULL THEN
            CREATE TEMP TABLE tmp_positions_moved (LIKE tracking.vehicle_positions) ON COMMIT DROP;
        END IF;
        TRUNCATE tmp_positions_moved;
        WITH moved AS (
            DELETE FROM tracking.vehicle_positions_default p
            WHERE p.recorded_at >= v_day::TIMESTAMP AT TIME ZONE 'UTC'
              AND p.recorded_at < (v_day + 1)::TIMESTAMP AT TIME ZONE 'UTC'
            RETURNING p.*
        )
        INSERT INTO tmp_positions_moved SELECT * FROM moved;

        EXECUTE format(
            'CREATE TABLE tracking.%I PARTITION OF tracking.vehicle_positions FOR VALUES FROM (%L) TO (%L)',
            v_name,
            to_char(v_day, 'YYYY-MM-DD') || ' 00:00:00+00',
            to_char(v_day + 1, 'YYYY-MM-DD') || ' 00:00:00+00');
        INSERT INTO tracking.vehicle_positions SELECT * FROM tmp_positions_moved;
        v_created := v_created + 1;
    END LOOP;

    IF p_retention_days IS NOT NULL THEN
        v_cutoff := v_today - p_retention_days;
        FOR v_part IN
            SELECT c.relname
            FROM pg_inherits i
            INNER JOIN pg_class c ON c.oid = i.inhrelid
            WHERE i.inhparent = 'tracking.vehicle_positions'::regclass
              AND c.relname ~ '^vehicle_positions_p[0-9]{8}$'
              AND to_date(substring(c.relname FROM 20), 'YYYYMMDD') < v_cutoff
        LOOP
            EXECUTE format('DROP TABLE tracking.%I', v_part.relname);
            v_dropped := v_dropped + 1;
        END LOOP;
        DELETE FROM tracking.vehicle_positions_default p
        WHERE p.recorded_at < v_cutoff::TIMESTAMP AT TIME ZONE 'UTC';
    END IF;

    RETURN QUERY SELECT v_created, v_dropped;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_positions_partitions(integer,integer) IS 'Creates the UTC-day partitions of vehicle_positions from today to today + p_ahead_days (moving any of their rows out of the default partition) and drops the ones older than p_retention_days; answers how many of each (TRACK-010)';
