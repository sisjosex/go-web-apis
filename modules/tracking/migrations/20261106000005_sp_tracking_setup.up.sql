-- TRACK-050 D2: the first-run assistant on the tracking home asks what the tenant has set up yet, in
-- order company → drivers → vehicles → routes → riders. One statement, five EXISTS, each stopping at
-- the first row found through the tenant index (companies, drivers) or the company / organization
-- index of the tenant's few companies and clients (vehicles, routes, riders): well under 2 ms, read once
-- when the home opens.

CREATE FUNCTION tracking.sp_tracking_setup(p_tenant_id UUID)
RETURNS TABLE(
    companies BOOLEAN,
    drivers   BOOLEAN,
    vehicles  BOOLEAN,
    routes    BOOLEAN,
    riders    BOOLEAN
) AS $$
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
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_tracking_setup(UUID) IS
'The setup assistant of the tracking home (TRACK-050 D2): whether the tenant has any company, driver, vehicle, route and rider yet';
