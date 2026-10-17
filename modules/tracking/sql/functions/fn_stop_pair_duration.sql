CREATE OR REPLACE FUNCTION tracking.fn_stop_pair_duration(p_tenant_id uuid, p_from_lat double precision, p_from_lng double precision, p_to_lat double precision, p_to_lng double precision, p_at timestamp with time zone)
 RETURNS TABLE(seconds integer, samples integer)
 LANGUAGE sql
 STABLE
AS $function$
    -- Each point is the tenant's nearest stop within 40 m (GIST); the pair's median over the last 90
    -- days in the weekday and 2-hour band of p_at when it has 3 samples, else over every band, else no
    -- row. One index range scan answers both bands.
    WITH places AS (
        SELECT
            (SELECT sp.id FROM tracking.stop_places sp
             WHERE sp.tenant_id = p_tenant_id
               AND ST_DWithin(sp.location, CAST(ST_SetSRID(ST_MakePoint(p_from_lng, p_from_lat), 4326) AS GEOGRAPHY), 40)
             ORDER BY sp.location <-> CAST(ST_SetSRID(ST_MakePoint(p_from_lng, p_from_lat), 4326) AS GEOGRAPHY)
             LIMIT 1) AS from_id,
            (SELECT sp.id FROM tracking.stop_places sp
             WHERE sp.tenant_id = p_tenant_id
               AND ST_DWithin(sp.location, CAST(ST_SetSRID(ST_MakePoint(p_to_lng, p_to_lat), 4326) AS GEOGRAPHY), 40)
             ORDER BY sp.location <-> CAST(ST_SetSRID(ST_MakePoint(p_to_lng, p_to_lat), 4326) AS GEOGRAPHY)
             LIMIT 1) AS to_id
    ),
    -- The band is read in the zone of the trips that recorded the pair: a sample's band is local time.
    band AS (
        SELECT p.from_id, p.to_id,
            CAST(EXTRACT(ISODOW FROM p_at AT TIME ZONE z.timezone) AS SMALLINT) AS weekday,
            CAST(EXTRACT(HOUR FROM p_at AT TIME ZONE z.timezone) AS SMALLINT) / 2 AS hour_band
        FROM places p
        CROSS JOIN LATERAL (
            SELECT t.timezone
            FROM tracking.stop_pair_samples s
            INNER JOIN tracking.trips t ON t.id = s.trip_id
            WHERE s.tenant_id = p_tenant_id AND s.from_stop_place_id = p.from_id AND s.to_stop_place_id = p.to_id
            ORDER BY s.observed_at DESC
            LIMIT 1
        ) z
    ),
    pair AS (
        SELECT
            percentile_cont(0.5) WITHIN GROUP (ORDER BY s.duration_s)
                FILTER (WHERE s.weekday = p.weekday AND s.hour_band = p.hour_band) AS band_median,
            COUNT(*) FILTER (WHERE s.weekday = p.weekday AND s.hour_band = p.hour_band) AS band_samples,
            percentile_cont(0.5) WITHIN GROUP (ORDER BY s.duration_s) AS any_median,
            COUNT(*) AS any_samples
        FROM band p
        INNER JOIN tracking.stop_pair_samples s
            ON s.tenant_id = p_tenant_id
           AND s.from_stop_place_id = p.from_id
           AND s.to_stop_place_id = p.to_id
           AND s.observed_at >= p_at - INTERVAL '90 days'
    )
    SELECT
        CAST(ROUND(CASE WHEN pr.band_samples >= 3 THEN pr.band_median ELSE pr.any_median END) AS INT),
        CAST(CASE WHEN pr.band_samples >= 3 THEN pr.band_samples ELSE pr.any_samples END AS INT)
    FROM pair pr
    WHERE pr.any_samples >= 3;
$function$;

COMMENT ON FUNCTION tracking.fn_stop_pair_duration(uuid,double precision,double precision,double precision,double precision,timestamp with time zone) IS 'Median observed seconds between the stops nearest two points: same weekday and 2-hour band when it has 3 samples, else any band (TRACK-066 D2)';
