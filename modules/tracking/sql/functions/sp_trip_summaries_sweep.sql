CREATE OR REPLACE FUNCTION tracking.sp_trip_summaries_sweep(p_tenant_id uuid, p_since date)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip_id UUID;
    v_count   INT := 0;
BEGIN
    FOR v_trip_id IN
        SELECT t.id
        FROM tracking.trips t
        LEFT JOIN tracking.trip_summaries s ON s.trip_id = t.id
        WHERE t.tenant_id = p_tenant_id
          AND t.service_date >= p_since
          AND t.status IN ('completed', 'cancelled')
          AND (s.trip_id IS NULL OR t.updated_at > s.rolled_at)
    LOOP
        PERFORM tracking.sp_trip_summarise(p_tenant_id, v_trip_id);
        v_count := v_count + 1;
    END LOOP;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_trip_summaries_sweep(uuid,date) IS 'Summarises the tenant''s trips ended since p_since that have no summary or changed after it was rolled; answers how many (TRACK-013 D1)';
