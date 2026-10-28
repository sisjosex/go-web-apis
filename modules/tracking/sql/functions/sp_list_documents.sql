CREATE OR REPLACE FUNCTION tracking.sp_list_documents(p_tenant_id uuid, p_subject_type character varying DEFAULT NULL::character varying, p_subject_id uuid DEFAULT NULL::uuid, p_expiring_within_days integer DEFAULT NULL::integer, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, tenant_id uuid, subject_type character varying, subject_id uuid, subject_name character varying, document_type_id uuid, type_name character varying, number character varying, issued_on date, expires_on date, file_name character varying, file_size bigint, file_ext character varying, notes text, status character varying, warned_at timestamp without time zone, created_at timestamp without time zone, updated_at timestamp without time zone, summary jsonb, total_count bigint)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    -- TRACK-076: the summary counts every document the other filters keep, so status is read for all
    -- of them; the page then narrows to one status.
    WITH f AS MATERIALIZED (
        SELECT
            d.id,
            d.expires_on,
            d.number,
            tracking.fn_document_subject_name(d.tenant_id, d.subject_type, d.subject_id) AS subject_name,
            CAST(t.name AS VARCHAR) AS type_name,
            CAST(CASE
                WHEN d.expires_on IS NULL THEN 'valid'
                WHEN d.expires_on < CURRENT_DATE THEN 'expired'
                WHEN d.expires_on <= CURRENT_DATE + t.warn_days_before THEN 'expiring'
                ELSE 'valid'
            END AS VARCHAR) AS status
        FROM tracking.compliance_documents d
        INNER JOIN tracking.document_types t ON t.id = d.document_type_id
        WHERE d.tenant_id = p_tenant_id
          AND (p_subject_type IS NULL OR p_subject_type = '' OR d.subject_type = p_subject_type)
          AND (p_subject_id IS NULL OR d.subject_id = p_subject_id)
          -- A document with no expiry never expires, so it is not part of "expiring within N days".
          AND (p_expiring_within_days IS NULL
               OR (d.expires_on IS NOT NULL AND d.expires_on <= CURRENT_DATE + p_expiring_within_days))
    ),
    s AS (
        SELECT f.*
        FROM f
        WHERE p_search IS NULL OR p_search = ''
           OR f.type_name    ILIKE '%' || p_search || '%'
           OR f.subject_name ILIKE '%' || p_search || '%'
           OR f.number       ILIKE '%' || p_search || '%'
    ),
    c AS (
        SELECT jsonb_build_object(
            'expired',  COUNT(*) FILTER (WHERE s.status = 'expired'),
            'expiring', COUNT(*) FILTER (WHERE s.status = 'expiring'),
            'valid',    COUNT(*) FILTER (WHERE s.status = 'valid')
        ) AS summary
        FROM s
    ),
    pg AS (
        SELECT s.id, s.subject_name, s.type_name, s.status, CAST(COUNT(*) OVER () AS BIGINT) AS total_count
        FROM s
        WHERE p_status IS NULL OR p_status = '' OR s.status = p_status
        -- Soonest first, which is the order the reader acts in; one with no expiry has nothing to act
        -- on and goes last. id breaks ties.
        ORDER BY s.expires_on ASC NULLS LAST, s.id ASC
        LIMIT  p_page_size
        OFFSET (p_page - 1) * p_page_size
    )
    SELECT
        d.id,
        d.tenant_id,
        CAST(d.subject_type AS VARCHAR),
        d.subject_id,
        pg.subject_name,
        d.document_type_id,
        pg.type_name,
        CAST(d.number AS VARCHAR),
        d.issued_on,
        d.expires_on,
        CAST(d.file_name AS VARCHAR),
        d.file_size,
        CAST(d.file_ext AS VARCHAR),
        d.notes,
        pg.status,
        d.warned_at,
        d.created_at,
        d.updated_at,
        c.summary,
        COALESCE(pg.total_count, 0)
    -- An empty page still answers one row, id NULL, so the summary keeps its counts.
    FROM c
    LEFT JOIN pg ON true
    LEFT JOIN tracking.compliance_documents d ON d.id = pg.id
    ORDER BY d.expires_on ASC NULLS LAST, d.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_documents(uuid,character varying,uuid,integer,integer,integer,character varying,character varying) IS 'One page of a tenant''s compliance documents by expiry, optionally narrowed to one subject, to what expires within N days, to a search over type name, subject name and number, and to one status; every row carries its type, its subject''s name, a status of valid/expiring/expired, the summary {expired, expiring, valid} over every filter but status (TRACK-076) and the total_count the filters match; an empty page is one row with id NULL that still carries the summary';
