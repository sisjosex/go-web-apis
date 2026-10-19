CREATE OR REPLACE FUNCTION tracking.fn_record_rider_access(p_tenant_id uuid, p_user_id uuid, p_level character varying, p_action character varying, p_rider_ids uuid[])
 RETURNS void
 LANGUAGE sql
AS $function$
    -- One row per user, rider, action and hour: a repeat read in the hour only counts (TRACK-063 D1).
    INSERT INTO tracking.rider_access_log AS l
        (tenant_id, rider_id, user_id, level, action, bucket, reads, first_at, last_at)
    SELECT p_tenant_id, ids.rider_id, p_user_id, p_level, p_action, date_trunc('hour', now()), 1, now(), now()
    FROM (SELECT DISTINCT unnest(p_rider_ids) AS rider_id) ids
    -- A staff read passes no user and is not logged (D2).
    WHERE p_user_id IS NOT NULL
    ON CONFLICT (rider_id, user_id, action, bucket)
    DO UPDATE SET reads = l.reads + 1, last_at = now();
$function$;

COMMENT ON FUNCTION tracking.fn_record_rider_access(uuid,uuid,character varying,character varying,uuid[]) IS 'Logs a portal or organization read of each of p_rider_ids in the current hour''s row, adding to its reads when it exists; a NULL p_user_id (staff) logs nothing (TRACK-063 D1, D2)';
