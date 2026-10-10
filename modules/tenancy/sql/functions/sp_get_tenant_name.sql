CREATE OR REPLACE FUNCTION tenancy.sp_get_tenant_name(p_tenant_id uuid)
 RETURNS character varying
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CAST(t.name AS VARCHAR) FROM tenancy.tenants t WHERE t.id = p_tenant_id;
$function$;
