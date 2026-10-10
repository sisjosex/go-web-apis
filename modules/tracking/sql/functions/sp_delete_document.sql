CREATE OR REPLACE FUNCTION tracking.sp_delete_document(p_tenant_id uuid, p_document_id uuid)
 RETURNS TABLE(file_ext character varying)
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
    DELETE FROM tracking.compliance_documents d
    WHERE d.id = p_document_id AND d.tenant_id = p_tenant_id
    RETURNING CAST(d.file_ext AS VARCHAR);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_delete_document(uuid,uuid) IS 'Deletes a document and returns the extension of the file it had, so the handler can remove that file; raises tracking.document.not-found';
