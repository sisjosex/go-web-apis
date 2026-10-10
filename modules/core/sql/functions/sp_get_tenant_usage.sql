CREATE OR REPLACE FUNCTION public.sp_get_tenant_usage(p_tenant_id uuid)
 RETURNS TABLE(feature character varying, used bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_riders   BIGINT := 0;
    v_vehicles BIGINT := 0;
    v_products BIGINT := 0;
BEGIN
    IF to_regclass('tracking.riders') IS NOT NULL THEN
        EXECUTE 'SELECT COUNT(*) FROM tracking.riders r
                 INNER JOIN tracking.organizations o ON o.id = r.organization_id
                 WHERE o.tenant_id = $1' INTO v_riders USING p_tenant_id;
        EXECUTE 'SELECT COUNT(*) FROM tracking.vehicles v
                 INNER JOIN tracking.transport_companies c ON c.id = v.company_id
                 WHERE c.tenant_id = $1' INTO v_vehicles USING p_tenant_id;
    END IF;
    IF to_regclass('inventory.products') IS NOT NULL THEN
        EXECUTE 'SELECT COUNT(*) FROM inventory.products p WHERE p.tenant_id = $1' INTO v_products USING p_tenant_id;
    END IF;

    RETURN QUERY VALUES
        (CAST('tracking_riders' AS VARCHAR), v_riders),
        (CAST('tracking_vehicles' AS VARCHAR), v_vehicles),
        (CAST('inventory_items' AS VARCHAR), v_products);
END;
$function$;

COMMENT ON FUNCTION sp_get_tenant_usage(uuid) IS 'Riders, vehicles and products a tenant holds in this database, for the billing usage meters (BILLING-001)';
