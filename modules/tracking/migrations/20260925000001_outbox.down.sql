-- Reverse of INFRA-001 step 4: the digest SP goes back to claiming only (20260922000003), and the
-- outbox goes with its relay's SPs and trigger.

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
    UPDATE tracking.compliance_documents d
       SET warned_at = CURRENT_TIMESTAMP
      FROM tracking.document_types t
     WHERE t.id = d.document_type_id
       AND d.tenant_id = p_tenant_id
       AND d.warned_at IS NULL
       AND d.expires_on IS NOT NULL
       AND d.expires_on <= CURRENT_DATE + t.warn_days_before
    RETURNING
        d.id,
        CAST(d.subject_type AS VARCHAR),
        tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id),
        CAST(t.name AS VARCHAR),
        CAST(d.number AS VARCHAR),
        d.expires_on,
        CAST(d.expires_on - CURRENT_DATE AS INT);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_raise_document_alerts(UUID) IS
'Claims every not-yet-warned document inside its type''s warning window and returns what to say about them (TRACK-016 D1); claiming and reporting are one statement, so a document is named exactly once however often the job runs';

DROP FUNCTION IF EXISTS tracking.sp_outbox_purge(INTERVAL);
DROP FUNCTION IF EXISTS tracking.sp_outbox_mark_published(BIGINT[]);
DROP FUNCTION IF EXISTS tracking.sp_outbox_claim(INT);
DROP TRIGGER IF EXISTS trg_outbox_notify ON tracking.outbox;
DROP FUNCTION IF EXISTS tracking.fn_outbox_notify();
DROP TABLE IF EXISTS tracking.outbox;
