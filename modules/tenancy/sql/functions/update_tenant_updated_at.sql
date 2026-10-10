CREATE OR REPLACE FUNCTION tenancy.update_tenant_updated_at()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS trigger_update_tenant_updated_at ON tenancy.tenants;
CREATE TRIGGER trigger_update_tenant_updated_at BEFORE UPDATE ON tenancy.tenants FOR EACH ROW EXECUTE FUNCTION tenancy.update_tenant_updated_at();
