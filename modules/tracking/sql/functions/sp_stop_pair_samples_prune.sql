CREATE OR REPLACE FUNCTION tracking.sp_stop_pair_samples_prune(p_tenant_id uuid, p_days integer)
 RETURNS integer
 LANGUAGE sql
AS $function$
    -- The median never reads past p_days; older samples only cost space.
    WITH deleted AS (
        DELETE FROM tracking.stop_pair_samples s
        WHERE s.tenant_id = p_tenant_id
          AND s.observed_at < now() - make_interval(days => p_days)
        RETURNING 1
    )
    SELECT CAST(COUNT(*) AS INT) FROM deleted;
$function$;

COMMENT ON FUNCTION tracking.sp_stop_pair_samples_prune(uuid,integer) IS 'Drops the tenant''s stop-to-stop samples older than p_days; answers how many (TRACK-066 D1)';
