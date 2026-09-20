-- TRACK-016 step 2: the expiry digest (D1) and the file columns a completed upload writes (D3).
--
-- "Exactly one alert per document" is the `warned_at IS NULL` predicate, not job bookkeeping: the
-- claim and the send are the same statement, so two jobs racing, a crash between them or a cron that
-- fires twice all end with one row claimed once. There is no schedule table, no outbox and no lock —
-- INFRA-001 swaps the send for an outbox row and nothing here changes.
--
-- A document is named as soon as it is inside its own type's warning window, which includes one that
-- has already expired: the digest is "act on these", not "these are about to happen".

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

-- Written only after the bytes are safely on disk, so a row never claims a file that is not there.
-- The upload replaces whatever the document had; the file itself is named after the document's id,
-- so a second upload overwrites the first and leaves nothing orphaned.
CREATE FUNCTION tracking.sp_set_document_file(
    p_tenant_id UUID,
    p_document_id UUID,
    p_file_name VARCHAR(255),
    p_file_size BIGINT,
    p_file_ext VARCHAR(10)
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    subject_type VARCHAR,
    subject_id UUID,
    subject_name VARCHAR,
    document_type_id UUID,
    type_name VARCHAR,
    number VARCHAR,
    issued_on DATE,
    expires_on DATE,
    file_name VARCHAR,
    file_size BIGINT,
    file_ext VARCHAR,
    notes TEXT,
    status VARCHAR,
    warned_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.compliance_documents d
        WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.compliance_documents d SET
        file_name  = p_file_name,
        file_size  = p_file_size,
        file_ext   = p_file_ext,
        updated_at = CURRENT_TIMESTAMP
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id;

    RETURN QUERY
    SELECT
        g.id,
        g.tenant_id,
        g.subject_type,
        g.subject_id,
        g.subject_name,
        g.document_type_id,
        g.type_name,
        g.number,
        g.issued_on,
        g.expires_on,
        g.file_name,
        g.file_size,
        g.file_ext,
        g.notes,
        g.status,
        g.warned_at,
        g.created_at,
        g.updated_at
    FROM tracking.sp_get_document(p_tenant_id, p_document_id) g;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_set_document_file(UUID, UUID, VARCHAR, BIGINT, VARCHAR) IS
'Records the file a document now carries, after the handler has written the bytes; raises tracking.document.not-found';
