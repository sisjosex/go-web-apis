CREATE OR REPLACE FUNCTION tracking.sp_update_document(p_tenant_id uuid, p_document_id uuid, p_number character varying, p_issued_on date, p_expires_on date, p_notes text)
 RETURNS TABLE(id uuid, tenant_id uuid, subject_type character varying, subject_id uuid, subject_name character varying, document_type_id uuid, type_name character varying, number character varying, issued_on date, expires_on date, file_name character varying, file_size bigint, file_ext character varying, notes text, status character varying, warned_at timestamp without time zone, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.compliance_documents d
        WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.compliance_documents d SET
        number     = COALESCE(NULLIF(TRIM(p_number), ''), d.number),
        issued_on  = COALESCE(p_issued_on, d.issued_on),
        expires_on = COALESCE(p_expires_on, d.expires_on),
        notes      = COALESCE(NULLIF(TRIM(p_notes), ''), d.notes),
        warned_at  = CASE
                         WHEN p_expires_on IS NOT NULL AND p_expires_on IS DISTINCT FROM d.expires_on
                         THEN NULL
                         ELSE d.warned_at
                     END,
        updated_at = CURRENT_TIMESTAMP
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id;

    -- The single read is the one projection every document response shares; naming the columns
    -- keeps the scan order identical to it without a second copy of the CASE.
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
$function$;
