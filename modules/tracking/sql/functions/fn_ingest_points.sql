CREATE OR REPLACE FUNCTION tracking.fn_ingest_points(p_tenant_id uuid, p_points jsonb)
 RETURNS TABLE(vehicle_id uuid, recorded_at timestamp with time zone, location geography, speed real, heading real, accuracy real, trip_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    WITH pts AS (
        SELECT
            CAST(p->>'vehicle_id' AS UUID) AS vehicle_id,
            CAST(p->>'recorded_at' AS TIMESTAMPTZ) AS recorded_at,
            CAST(ST_SetSRID(ST_MakePoint(CAST(p->>'lng' AS FLOAT8), CAST(p->>'lat' AS FLOAT8)), 4326) AS GEOGRAPHY) AS location,
            CAST(p->>'speed' AS REAL) AS speed,
            CAST(p->>'heading' AS REAL) AS heading,
            CAST(p->>'accuracy' AS REAL) AS accuracy
        FROM jsonb_array_elements(p_points) p
    )
    SELECT pts.vehicle_id, pts.recorded_at, pts.location, pts.speed, pts.heading, pts.accuracy, a.trip_id
    FROM pts
    LEFT JOIN LATERAL (
        SELECT t.id AS trip_id
        FROM tracking.trips t
        WHERE t.vehicle_id = pts.vehicle_id
          AND t.status = 'in_progress'
          AND t.tenant_id = p_tenant_id
        ORDER BY t.started_at DESC
        LIMIT 1
    ) a ON true;
$function$;

COMMENT ON FUNCTION tracking.fn_ingest_points(uuid,jsonb) IS 'A batch of GPS points as rows, each with its vehicle''s trip in progress (TRACK-010)';
