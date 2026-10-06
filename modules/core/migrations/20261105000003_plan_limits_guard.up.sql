-- BILLING-001 D2: plan limits checked where the row is written, in every database (core migrations
-- also run on a dedicated tenant's).
--
-- The API knows the tenant's limit (its plan, cached 60 s) and the caller puts the guard in the same
-- statement as the create:
--
--   SELECT r.* FROM public.fn_within_limit($tenant, <count subquery>, $limit, 'tracking_riders') g(id),
--          LATERAL tracking.sp_create_rider(g.id, ...) r
--
-- LATERAL makes the create depend on the guard's row, so the guard runs first and a refusal aborts the
-- whole statement: no extra round trip, one snapshot for the count and the insert.

CREATE OR REPLACE FUNCTION public.fn_within_limit(
    p_tenant_id UUID,
    p_used      BIGINT,
    p_limit     INT,
    p_feature   TEXT
)
RETURNS UUID AS $$
BEGIN
    IF p_limit IS NOT NULL AND p_limit >= 0 AND p_used >= p_limit THEN
        RAISE EXCEPTION 'billing.limit-reached'
            USING ERRCODE = 'P0001',
                  DETAIL = json_build_object('feature', p_feature, 'limit', p_limit, 'used', p_used)::TEXT;
    END IF;
    RETURN p_tenant_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION public.fn_within_limit(UUID, BIGINT, INT, TEXT) IS
'Answers p_tenant_id while p_used is under p_limit (NULL or -1: unlimited); else raises billing.limit-reached with {feature, limit, used} in DETAIL (BILLING-001 D2)';

-- What a tenant uses of each limited feature, from the tables of this database. A module this database
-- does not hold counts 0, so one function serves the shared database and a dedicated one alike.
CREATE OR REPLACE FUNCTION public.sp_get_tenant_usage(p_tenant_id UUID)
RETURNS TABLE(feature VARCHAR, used BIGINT) AS $$
DECLARE
    v_riders   BIGINT := 0;
    v_vehicles BIGINT := 0;
    v_products BIGINT := 0;
BEGIN
    IF to_regclass('tracking.riders') IS NOT NULL THEN
        EXECUTE 'SELECT COUNT(*) FROM tracking.riders r
                 INNER JOIN tracking.transport_companies c ON c.id = r.company_id
                 WHERE c.tenant_id = $1' INTO v_riders USING p_tenant_id;
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
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION public.sp_get_tenant_usage(UUID) IS
'Riders, vehicles and products a tenant holds in this database, for the billing usage meters (BILLING-001)';
