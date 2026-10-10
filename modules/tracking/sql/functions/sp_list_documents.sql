CREATE OR REPLACE FUNCTION tracking.sp_list_documents(p_tenant_id uuid, p_subject_type character varying DEFAULT NULL::character varying, p_subject_id uuid DEFAULT NULL::uuid, p_expiring_within_days integer DEFAULT NULL::integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, tenant_id uuid, subject_type character varying, subject_id uuid, subject_name character varying, document_type_id uuid, type_name character varying, number character varying, issued_on date, expires_on date, file_name character varying, file_size bigint, file_ext character varying, notes text, status character varying, warned_at timestamp without time zone, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
        d.updated_at,
        CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.compliance_documents d
    INNER JOIN tracking.document_types t ON t.id = d.document_type_id
    WHERE d.tenant_id = p_tenant_id
      AND (p_subject_type IS NULL OR p_subject_type = '' OR d.subject_type = p_subject_type)
      AND (p_subject_id IS NULL OR d.subject_id = p_subject_id)
      -- A document with no expiry never expires, so it is not part of "expiring within N days".
      AND (p_expiring_within_days IS NULL
           OR (d.expires_on IS NOT NULL AND d.expires_on <= CURRENT_DATE + p_expiring_within_days))
    -- Soonest first, which is the order the reader acts in; one with no expiry has nothing to act on
    -- and goes last. d.id breaks ties.
    ORDER BY d.expires_on ASC NULLS LAST, d.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_documents(uuid,character varying,uuid,integer,integer,integer) IS 'One page of a tenant''s compliance documents by expiry, optionally narrowed to one subject or to what expires within N days; every row carries its type, its subject''s name, a status of valid/expiring/expired and the total_count the filters match';
