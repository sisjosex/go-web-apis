CREATE OR REPLACE FUNCTION tracking.sp_usage_for_day(p_tenant_id uuid, p_day date)
 RETURNS TABLE(vehicles_active integer, trips_completed integer, positions bigint, riders_boarded integer)
 LANGUAGE sql
 STABLE
AS $function$
    WITH fleet AS (
        SELECT v.id
        FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE tc.tenant_id = p_tenant_id
    ),
    -- The UTC day's partition only: the bounds are constants for the planner.
    points AS (
        SELECT vp.vehicle_id, COUNT(*) AS n
        FROM tracking.vehicle_positions vp
        WHERE vp.recorded_at >= CAST(p_day AS TIMESTAMP) AT TIME ZONE 'UTC'
          AND vp.recorded_at < CAST(p_day + 1 AS TIMESTAMP) AT TIME ZONE 'UTC'
          AND vp.vehicle_id IN (SELECT f.id FROM fleet f)
        GROUP BY vp.vehicle_id
    ),
    driven AS (
        SELECT s.vehicle_id
        FROM tracking.trip_summaries s
        WHERE s.tenant_id = p_tenant_id
          AND s.service_date = p_day
          AND s.status = 'completed'
          AND s.vehicle_id IS NOT NULL
    )
    SELECT
        CAST((SELECT COUNT(*) FROM (SELECT d.vehicle_id FROM driven d UNION SELECT p.vehicle_id FROM points p) a) AS INT),
        CAST((
            SELECT COUNT(*) FROM tracking.trip_summaries s
            WHERE s.tenant_id = p_tenant_id AND s.service_date = p_day AND s.status = 'completed'
        ) AS INT),
        CAST(COALESCE((SELECT SUM(p.n) FROM points p), 0) AS BIGINT),
        CAST((
            SELECT COUNT(*) FROM tracking.rider_day_attendance a
            WHERE a.tenant_id = p_tenant_id AND a.service_date = p_day AND a.status = 'boarded'
        ) AS INT);
$function$;

COMMENT ON FUNCTION tracking.sp_usage_for_day(uuid,date) IS 'What the tenant used on one day: vehicles that completed a trip or sent a GPS point that UTC day, trips completed, points and riders boarded (TRACK-013 D3)';
