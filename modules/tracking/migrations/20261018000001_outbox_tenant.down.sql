-- Reverses INFRA-009 D1: the outbox without its tenant column, the claim as INFRA-001 wrote it.

DROP FUNCTION IF EXISTS tracking.sp_outbox_claim(INT);

CREATE FUNCTION tracking.sp_outbox_claim(
    p_limit INT
)
RETURNS TABLE(
    id BIGINT,
    topic TEXT,
    payload JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT o.id, o.topic, o.payload
      FROM tracking.outbox o
     WHERE o.published_at IS NULL
     ORDER BY o.id
     LIMIT p_limit
       FOR UPDATE SKIP LOCKED;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_outbox_claim(INT) IS
'Locks and returns up to p_limit unpublished outbox rows, oldest first, skipping rows another relay holds; call it inside the transaction that marks them';

DROP TRIGGER IF EXISTS trg_outbox_tenant ON tracking.outbox;
DROP FUNCTION IF EXISTS tracking.fn_outbox_tenant();
ALTER TABLE tracking.outbox DROP COLUMN IF EXISTS tenant_id;
