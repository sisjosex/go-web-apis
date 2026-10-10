CREATE OR REPLACE FUNCTION tracking.sp_create_document_type(p_tenant_id uuid, p_code character varying, p_name character varying, p_applies_to character varying, p_warn_days_before integer, p_blocks_service boolean, p_is_active boolean)
 RETURNS TABLE(id uuid, tenant_id uuid, code character varying, name character varying, applies_to character varying, warn_days_before integer, blocks_service boolean, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF EXISTS (
        SELECT 1 FROM tracking.document_types t
        WHERE t.tenant_id = p_tenant_id AND t.code = UPPER(TRIM(p_code))
    ) THEN
        RAISE EXCEPTION 'document-type.code-already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.document_types (
        tenant_id, code, name, applies_to, warn_days_before, blocks_service, is_active
    )
    VALUES (
        p_tenant_id,
        UPPER(TRIM(p_code)),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_applies_to), ''), 'both'),
        COALESCE(p_warn_days_before, 30),
        COALESCE(p_blocks_service, false),
        COALESCE(p_is_active, true)
    )
    RETURNING
        tracking.document_types.id,
        tracking.document_types.tenant_id,
        CAST(tracking.document_types.code AS VARCHAR),
        CAST(tracking.document_types.name AS VARCHAR),
        CAST(tracking.document_types.applies_to AS VARCHAR),
        tracking.document_types.warn_days_before,
        tracking.document_types.blocks_service,
        tracking.document_types.is_active,
        tracking.document_types.created_at,
        tracking.document_types.updated_at;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_document_type(uuid,character varying,character varying,character varying,integer,boolean,boolean) IS 'Creates a document type for one tenant; the code is upper-cased and unique per tenant, raising tracking.document-type.code-already-exists';
