CREATE OR REPLACE FUNCTION tracking.sp_outbox_claim(p_limit integer)
 RETURNS TABLE(id bigint, tenant_id uuid, topic text, payload jsonb)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT o.id, o.tenant_id, o.topic, o.payload
      FROM tracking.outbox o
     WHERE o.published_at IS NULL
     ORDER BY o.id
     LIMIT p_limit
       FOR UPDATE SKIP LOCKED;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_outbox_claim(integer) IS 'Locks and returns up to p_limit unpublished outbox rows with their tenant, oldest first, skipping rows another relay holds; call it inside the transaction that marks them';
