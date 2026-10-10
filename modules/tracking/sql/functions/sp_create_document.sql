CREATE OR REPLACE FUNCTION tracking.sp_create_document(p_tenant_id uuid, p_subject_type character varying, p_subject_id uuid, p_document_type_id uuid, p_number character varying, p_issued_on date, p_expires_on date, p_notes text)
 RETURNS TABLE(id uuid, tenant_id uuid, subject_type character varying, subject_id uuid, subject_name character varying, document_type_id uuid, type_name character varying, number character varying, issued_on date, expires_on date, file_name character varying, file_size bigint, file_ext character varying, notes text, status character varying, warned_at timestamp without time zone, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_applies_to VARCHAR;
    v_document_id UUID;
BEGIN
    SELECT t.applies_to INTO v_applies_to
    FROM tracking.document_types t
    WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id;

    IF v_applies_to IS NULL THEN
        RAISE EXCEPTION 'document-type.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF v_applies_to NOT IN (p_subject_type, 'both') THEN
        RAISE EXCEPTION 'document.type-not-applicable' USING ERRCODE = 'P0001';
    END IF;

    -- No FK reaches the subject (subject_type decides the table), so this is the check that keeps a
    -- document from being filed against a driver or vehicle that is not this tenant's, or not there.
    IF tracking.fn_document_subject_name(p_tenant_id, p_subject_type, p_subject_id) IS NULL THEN
        RAISE EXCEPTION 'document.subject-not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.compliance_documents (
        tenant_id, subject_type, subject_id, document_type_id, number, issued_on, expires_on, notes
    )
    VALUES (
        p_tenant_id,
        p_subject_type,
        p_subject_id,
        p_document_type_id,
        NULLIF(TRIM(p_number), ''),
        p_issued_on,
        p_expires_on,
        NULLIF(TRIM(p_notes), '')
    )
    RETURNING tracking.compliance_documents.id INTO v_document_id;

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
    FROM tracking.sp_get_document(p_tenant_id, v_document_id) g;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_document(uuid,character varying,uuid,uuid,character varying,date,date,text) IS 'Files a document against one vehicle or driver; raises tracking.document-type.not-found, tracking.document.type-not-applicable or tracking.document.subject-not-found';
