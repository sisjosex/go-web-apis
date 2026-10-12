CREATE OR REPLACE FUNCTION tracking.sp_outbox_lag()
 RETURNS TABLE(unpublished_rows bigint, oldest_unpublished_seconds double precision)
 LANGUAGE sql
 STABLE
AS $function$
    -- One pass over the unpublished tail (idx_outbox_unpublished's predicate), never the 7 days of
    -- published rows. min(created_at), not the lowest id: created_at is its transaction's start, so a
    -- long transaction's row can be older than a lower id (INFRA-018).
    SELECT count(*),
           COALESCE(EXTRACT(EPOCH FROM now() - min(o.created_at))::double precision, 0)
      FROM tracking.outbox o
     WHERE o.published_at IS NULL;
$function$;

COMMENT ON FUNCTION tracking.sp_outbox_lag() IS 'The relay''s backlog in this database: unpublished outbox rows and the age in seconds of the oldest, 0 when none waits (INFRA-018)';
