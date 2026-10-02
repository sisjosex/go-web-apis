-- INFRA-009 D1: every outbox row says whose it is. The table assumed a database per tenant (INFRA-001
-- D3); INFRA-006 D2 put every tenant in the shared `web`, where the relay could not tell them apart.
--
-- A BEFORE INSERT trigger fills tenant_id from what each row already carries — document.alerts names
-- its tenant, every other topic names a route — so none of the SPs writing the outbox changes: one
-- primary-key lookup per event. A row that names neither is a bug and fails its transaction rather
-- than reach a relay with no owner.

ALTER TABLE tracking.outbox ADD COLUMN tenant_id UUID;

-- Rows written before the column: the same rule as the trigger below. One whose route is gone has
-- nothing left to act on.
UPDATE tracking.outbox o
   SET tenant_id = COALESCE(
       CAST(o.payload->>'tenant_id' AS UUID),
       (SELECT tc.tenant_id
          FROM tracking.routes r
          INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
         WHERE r.id = CAST(o.payload->>'route_id' AS UUID)));
DELETE FROM tracking.outbox o WHERE o.tenant_id IS NULL;

ALTER TABLE tracking.outbox ALTER COLUMN tenant_id SET NOT NULL;

COMMENT ON COLUMN tracking.outbox.tenant_id IS
'Whose event this is; the relay publishes the task as <tenant_id>:<id>, so tenants sharing a database never mix (INFRA-009)';

-- The row's tenant, when the writer did not set it: its payload's tenant_id, else its route's tenant.
CREATE FUNCTION tracking.fn_outbox_tenant()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.tenant_id IS NULL THEN
        NEW.tenant_id := COALESCE(
            CAST(NEW.payload->>'tenant_id' AS UUID),
            (SELECT tc.tenant_id
               FROM tracking.routes r
               INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
              WHERE r.id = CAST(NEW.payload->>'route_id' AS UUID)));
    END IF;
    IF NEW.tenant_id IS NULL THEN
        RAISE EXCEPTION 'outbox.tenant' USING ERRCODE = 'P0001',
            DETAIL = format('topic %s names no tenant_id or known route_id', NEW.topic);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_outbox_tenant
    BEFORE INSERT ON tracking.outbox
    FOR EACH ROW EXECUTE FUNCTION tracking.fn_outbox_tenant();

-- The claim answers the tenant with each row.
DROP FUNCTION IF EXISTS tracking.sp_outbox_claim(INT);

CREATE FUNCTION tracking.sp_outbox_claim(
    p_limit INT
)
RETURNS TABLE(
    id BIGINT,
    tenant_id UUID,
    topic TEXT,
    payload JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT o.id, o.tenant_id, o.topic, o.payload
      FROM tracking.outbox o
     WHERE o.published_at IS NULL
     ORDER BY o.id
     LIMIT p_limit
       FOR UPDATE SKIP LOCKED;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_outbox_claim(INT) IS
'Locks and returns up to p_limit unpublished outbox rows with their tenant, oldest first, skipping rows another relay holds; call it inside the transaction that marks them';
