CREATE OR REPLACE FUNCTION tracking.fn_route_company_check(p_tenant_id uuid, p_company_id uuid, p_vehicle_id uuid, p_driver_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- The company is the tenant's (both callers check it first), so matching it scopes the vehicle;
    -- the driver row carries its own tenant_id.
    IF p_vehicle_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.vehicles v
        INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
        WHERE v.id = p_vehicle_id AND v.company_id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'vehicle_id';
    END IF;

    IF p_driver_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.company_id = p_company_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.company-mismatch' USING ERRCODE = 'P0001', DETAIL = 'default_driver_id';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_route_company_check(uuid,uuid,uuid,uuid) IS 'Raises route.company-mismatch, DETAIL naming the field, when a route''s vehicle or default driver belongs to another carrier (TRACK-002)';
