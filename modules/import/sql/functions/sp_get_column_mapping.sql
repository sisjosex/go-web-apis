CREATE OR REPLACE FUNCTION import.sp_get_column_mapping(p_tenant_id uuid, p_resource character varying, p_header_signature character)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    SELECT m.mapping
    FROM import.column_mappings m
    WHERE m.tenant_id = p_tenant_id
      AND m.resource = p_resource
      AND m.header_signature = p_header_signature
$function$;

COMMENT ON FUNCTION import.sp_get_column_mapping(uuid,character varying,character) IS 'The remembered {folded heading: field} of a file shape for a resource, or NULL (IMPORT-002 D3)';
