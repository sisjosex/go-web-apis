CREATE OR REPLACE FUNCTION tenancy.update_tenant_module_updated_at()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS trigger_tenant_modules_updated_at ON tenancy.tenant_modules;
CREATE TRIGGER trigger_tenant_modules_updated_at BEFORE UPDATE ON tenancy.tenant_modules FOR EACH ROW EXECUTE FUNCTION tenancy.update_tenant_module_updated_at();
