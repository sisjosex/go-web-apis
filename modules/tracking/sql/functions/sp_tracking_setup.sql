CREATE OR REPLACE FUNCTION tracking.sp_tracking_setup(p_tenant_id uuid)
 RETURNS TABLE(companies boolean, drivers boolean, vehicles boolean, routes boolean, riders boolean)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        EXISTS (SELECT 1 FROM tracking.transport_companies tc WHERE tc.tenant_id = p_tenant_id),
        EXISTS (SELECT 1 FROM tracking.drivers d WHERE d.tenant_id = p_tenant_id),
        EXISTS (
            SELECT 1 FROM tracking.vehicles v
            INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
            WHERE tc.tenant_id = p_tenant_id
        ),
        EXISTS (
            SELECT 1 FROM tracking.routes r
            INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
            WHERE tc.tenant_id = p_tenant_id
        ),
        EXISTS (
            SELECT 1 FROM tracking.riders rd
            INNER JOIN tracking.organizations o ON o.id = rd.organization_id
            WHERE o.tenant_id = p_tenant_id
        );
$function$;

COMMENT ON FUNCTION tracking.sp_tracking_setup(uuid) IS 'The setup assistant of the tracking home (TRACK-050 D2): whether the tenant has any company, driver, vehicle, route and rider yet';
