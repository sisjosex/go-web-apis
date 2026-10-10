CREATE OR REPLACE FUNCTION tracking.sp_outbox_mark_published(p_ids bigint[])
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    UPDATE tracking.outbox o
       SET published_at = now()
     WHERE o.id = ANY(p_ids);
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_outbox_mark_published(bigint[]) IS 'Stamps published_at on the rows the relay has enqueued';
