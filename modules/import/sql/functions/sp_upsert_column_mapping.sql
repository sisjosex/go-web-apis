CREATE OR REPLACE FUNCTION import.sp_upsert_column_mapping(p_tenant_id uuid, p_resource character varying, p_header_signature character, p_mapping jsonb)
 RETURNS void
 LANGUAGE sql
AS $function$
    INSERT INTO import.column_mappings (tenant_id, resource, header_signature, mapping, updated_at)
    VALUES (p_tenant_id, p_resource, p_header_signature, p_mapping, now())
    ON CONFLICT (tenant_id, resource, header_signature)
    DO UPDATE SET mapping = EXCLUDED.mapping, updated_at = now()
$function$;

COMMENT ON FUNCTION import.sp_upsert_column_mapping(uuid,character varying,character,jsonb) IS 'Remembers a file shape''s {folded heading: field} for a resource, replacing the previous one (IMPORT-002 D3)';
