CREATE OR REPLACE FUNCTION tracking.sp_outbox_purge(p_older_than interval)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    DELETE FROM tracking.outbox o
     WHERE o.published_at < now() - p_older_than;
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_outbox_purge(interval) IS 'Deletes rows published longer ago than p_older_than; an unpublished row is never purged';
