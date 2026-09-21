-- INFRA-001 step 4: the transactional outbox (D3) and the first producer on it (D4).
--
-- A side effect that must survive a crash is written as a row in the same transaction as the change
-- that causes it: the row exists if and only if the change committed. The relay (core/jobs) turns each
-- row into one asynq task and stamps published_at. A rolled-back transaction leaves no row and sends
-- no NOTIFY — PostgreSQL delivers notifications on commit only — so it is never published.
--
-- No tenant_id column: every tenant has its own database, and the relay knows whose database it is
-- reading. The partial index keeps the relay's claim an index scan over the unpublished tail only,
-- however many published rows wait for the 7-day purge.

CREATE TABLE tracking.outbox (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON tracking.outbox (id) WHERE published_at IS NULL;

COMMENT ON TABLE tracking.outbox IS
'Side effects written in the transaction that causes them; the worker''s relay publishes each row as an asynq task outbox:<topic> and stamps published_at (INFRA-001 D3)';

-- One NOTIFY per statement, not per row: the relay drains everything unpublished on each wake-up,
-- so a batch insert needs one signal. The payload is empty for the same reason.
CREATE FUNCTION tracking.fn_outbox_notify()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('tracking_outbox', '');
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_outbox_notify
    AFTER INSERT ON tracking.outbox
    FOR EACH STATEMENT EXECUTE FUNCTION tracking.fn_outbox_notify();

-- The relay's three statements. The claim locks what it returns until the caller's transaction ends,
-- and SKIP LOCKED lets several workers drain one tenant without publishing a row twice.
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

CREATE FUNCTION tracking.sp_outbox_mark_published(
    p_ids BIGINT[]
)
RETURNS INT AS $$
DECLARE
    v_count INT;
BEGIN
    UPDATE tracking.outbox o
       SET published_at = now()
     WHERE o.id = ANY(p_ids);
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_outbox_mark_published(BIGINT[]) IS
'Stamps published_at on the rows the relay has enqueued';

CREATE FUNCTION tracking.sp_outbox_purge(
    p_older_than INTERVAL
)
RETURNS INT AS $$
DECLARE
    v_count INT;
BEGIN
    DELETE FROM tracking.outbox o
     WHERE o.published_at < now() - p_older_than;
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_outbox_purge(INTERVAL) IS
'Deletes rows published longer ago than p_older_than; an unpublished row is never purged';

-- D4: the document digest moves onto the outbox. The claim is unchanged; the same statement now also
-- writes one outbox row per pass carrying everything the mail needs, so the handler reads nothing
-- back and a pass that claims nothing writes nothing. The data-modifying CTE runs exactly once
-- whether or not the outer query reads it. DROP + CREATE (AGENTS.md), result shape unchanged.

DROP FUNCTION IF EXISTS tracking.sp_raise_document_alerts(UUID);

CREATE FUNCTION tracking.sp_raise_document_alerts(
    p_tenant_id UUID
)
RETURNS TABLE(
    id UUID,
    subject_type VARCHAR,
    subject_name VARCHAR,
    type_name VARCHAR,
    number VARCHAR,
    expires_on DATE,
    days_left INT
) AS $$
BEGIN
    RETURN QUERY
    WITH claimed AS (
        UPDATE tracking.compliance_documents d
           SET warned_at = CURRENT_TIMESTAMP
          FROM tracking.document_types t
         WHERE t.id = d.document_type_id
           AND d.tenant_id = p_tenant_id
           AND d.warned_at IS NULL
           AND d.expires_on IS NOT NULL
           AND d.expires_on <= CURRENT_DATE + t.warn_days_before
        RETURNING
            d.id AS id,
            CAST(d.subject_type AS VARCHAR) AS subject_type,
            tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id) AS subject_name,
            CAST(t.name AS VARCHAR) AS type_name,
            CAST(d.number AS VARCHAR) AS number,
            d.expires_on AS expires_on,
            CAST(d.expires_on - CURRENT_DATE AS INT) AS days_left
    ), queued AS (
        INSERT INTO tracking.outbox (topic, payload)
        SELECT 'document.alerts',
               jsonb_build_object(
                   'tenant_id', p_tenant_id,
                   'documents', jsonb_agg(to_jsonb(c) ORDER BY c.expires_on, c.id)
               )
          FROM claimed c
        HAVING count(*) > 0
    )
    SELECT c.id, c.subject_type, c.subject_name, c.type_name, c.number, c.expires_on, c.days_left
      FROM claimed c
     ORDER BY c.expires_on, c.id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_raise_document_alerts(UUID) IS
'Claims every not-yet-warned document inside its type''s warning window, writes one document.alerts outbox row for the pass and returns the claimed documents (TRACK-016 D1, INFRA-001 D4); a document is named exactly once however often the job runs';
