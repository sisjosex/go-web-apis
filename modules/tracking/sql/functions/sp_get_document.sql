CREATE OR REPLACE FUNCTION tracking.sp_get_document(p_tenant_id uuid, p_document_id uuid)
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

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        CAST(d.subject_type AS VARCHAR),
        d.subject_id,
        tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id),
        d.document_type_id,
        CAST(t.name AS VARCHAR),
        CAST(d.number AS VARCHAR),
        d.issued_on,
        d.expires_on,
        CAST(d.file_name AS VARCHAR),
        d.file_size,
        CAST(d.file_ext AS VARCHAR),
        d.notes,
        CAST(CASE
            WHEN d.expires_on IS NULL THEN 'valid'
            WHEN d.expires_on < CURRENT_DATE THEN 'expired'
            WHEN d.expires_on <= CURRENT_DATE + t.warn_days_before THEN 'expiring'
            ELSE 'valid'
        END AS VARCHAR),
        d.warned_at,
        d.created_at,
        d.updated_at
    FROM tracking.compliance_documents d
    INNER JOIN tracking.document_types t ON t.id = d.document_type_id
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id;
END;
$function$;
