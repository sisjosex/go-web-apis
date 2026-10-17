CREATE OR REPLACE FUNCTION tracking.fn_service_block_reason(p_tenant_id uuid, p_subject_type character varying, p_subject_id uuid)
 RETURNS character varying
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CAST(t.name AS VARCHAR)
    FROM tracking.compliance_documents d
    INNER JOIN tracking.document_types t ON t.id = d.document_type_id
    WHERE d.tenant_id = p_tenant_id
      AND d.subject_type = p_subject_type
      AND d.subject_id = p_subject_id
      AND t.blocks_service
      AND t.is_active
      AND d.expires_on IS NOT NULL
      AND d.expires_on < CURRENT_DATE
    ORDER BY d.expires_on, t.name
    LIMIT 1;
$function$;

COMMENT ON FUNCTION tracking.fn_service_block_reason(uuid,character varying,uuid) IS 'The type name of the earliest-expired document of a blocks_service type standing against this vehicle or driver, NULL when none — the reason an assignment or a start is refused and a trip is flagged (TRACK-064 D3)';
