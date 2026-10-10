CREATE OR REPLACE FUNCTION tracking.sp_raise_document_alerts(p_tenant_id uuid)
 RETURNS TABLE(id uuid, subject_type character varying, subject_name character varying, type_name character varying, number character varying, expires_on date, days_left integer)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.sp_raise_document_alerts(uuid) IS 'Claims every not-yet-warned document inside its type''s warning window, writes one document.alerts outbox row for the pass and returns the claimed documents (TRACK-016 D1, INFRA-001 D4); a document is named exactly once however often the job runs';
