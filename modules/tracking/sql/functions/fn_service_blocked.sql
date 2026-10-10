CREATE OR REPLACE FUNCTION tracking.fn_service_blocked(p_tenant_id uuid, p_subject_type character varying, p_subject_id uuid)
 RETURNS boolean
 LANGUAGE sql
 STABLE
AS $function$
    SELECT EXISTS (
        SELECT 1
        FROM tracking.compliance_documents d
        INNER JOIN tracking.document_types t ON t.id = d.document_type_id
        WHERE d.tenant_id = p_tenant_id
          AND d.subject_type = p_subject_type
          AND d.subject_id = p_subject_id
          AND t.blocks_service
          AND t.is_active
          AND d.expires_on IS NOT NULL
          AND d.expires_on < CURRENT_DATE
    );
$function$;

COMMENT ON FUNCTION tracking.fn_service_blocked(uuid,character varying,uuid) IS 'Whether an expired document of a blocks_service type stands against this vehicle or driver (TRACK-016 D2); the lists show it and TRACK-008''s assignment refuses on it';
