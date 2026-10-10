CREATE OR REPLACE FUNCTION tracking.sp_update_document_type(p_tenant_id uuid, p_document_type_id uuid, p_code character varying, p_name character varying, p_applies_to character varying, p_warn_days_before integer, p_blocks_service boolean, p_is_active boolean)
 RETURNS TABLE(id uuid, tenant_id uuid, code character varying, name character varying, applies_to character varying, warn_days_before integer, blocks_service boolean, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'document-type.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_code IS NOT NULL AND TRIM(p_code) <> '' AND EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.tenant_id = p_tenant_id
          AND t.code = UPPER(TRIM(p_code))
          AND t.id <> p_document_type_id
    ) THEN
        RAISE EXCEPTION 'document-type.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    UPDATE tracking.document_types t SET
        code             = COALESCE(NULLIF(UPPER(TRIM(p_code)), ''), t.code),
        name             = COALESCE(NULLIF(TRIM(p_name), ''), t.name),
        applies_to       = COALESCE(NULLIF(TRIM(p_applies_to), ''), t.applies_to),
        warn_days_before = COALESCE(p_warn_days_before, t.warn_days_before),
        blocks_service   = COALESCE(p_blocks_service, t.blocks_service),
        is_active        = COALESCE(p_is_active, t.is_active),
        updated_at       = CURRENT_TIMESTAMP
    WHERE t.id = p_document_type_id AND t.tenant_id = p_tenant_id
    RETURNING
        t.id,
        t.tenant_id,
        CAST(t.code AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(t.applies_to AS VARCHAR),
        t.warn_days_before,
        t.blocks_service,
        t.is_active,
        t.created_at,
        t.updated_at;
END;
$function$;
