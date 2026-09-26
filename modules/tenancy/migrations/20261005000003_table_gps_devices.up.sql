-- TRACK-010 step 2: a GPS device's credential.
--
-- A device request carries no X-Tenant-Slug, so the token itself says whose it is: it reads
-- `<tenant_id>.<secret>`, and the platform database keeps the sha256 of the whole token next to the
-- tenant and the vehicle (the vehicle lives in the tenant's own database, hence no foreign key). Both
-- SPs are scoped by that tenant like any other. One token per vehicle — issuing again rotates it, and
-- the old one stops working at once. The token is printed once by `cli tenant -gps-device` and never
-- stored.
--
-- Resolving a token is the device request's one platform query: it answers the tenant as the tenant
-- middleware needs it and stamps last_seen_at at most once a minute, so a device posting every five
-- seconds writes the row twelve times less often than it reads it.

CREATE TABLE tenancy.gps_devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash   BYTEA NOT NULL UNIQUE,
    tenant_id    UUID NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    vehicle_id   UUID NOT NULL,
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uk_gps_devices_vehicle UNIQUE (tenant_id, vehicle_id)
);

COMMENT ON TABLE tenancy.gps_devices IS
'One GPS device credential per vehicle: the sha256 of its token, whose tenant and vehicle it posts for (TRACK-010)';

CREATE FUNCTION tenancy.sp_gps_device_issue(
    p_tenant_id UUID,
    p_vehicle_id UUID,
    p_token_hash BYTEA
)
RETURNS VOID AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.gps_devices AS d (token_hash, tenant_id, vehicle_id)
    VALUES (p_token_hash, p_tenant_id, p_vehicle_id)
    ON CONFLICT ON CONSTRAINT uk_gps_devices_vehicle DO UPDATE
    SET token_hash = EXCLUDED.token_hash, last_seen_at = NULL, created_at = now();
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tenancy.sp_gps_device_issue(UUID, UUID, BYTEA) IS
'Stores the token hash of the vehicle''s GPS device, replacing its previous token; raises tenant.not-found (TRACK-010)';

CREATE FUNCTION tenancy.sp_gps_device_resolve(
    p_tenant_id UUID,
    p_token_hash BYTEA
)
RETURNS TABLE(
    tenant_id UUID,
    slug VARCHAR,
    name VARCHAR,
    database_url TEXT,
    schema_name VARCHAR,
    is_active BOOLEAN,
    is_suspended BOOLEAN,
    vehicle_id UUID
) AS $$
BEGIN
    UPDATE tenancy.gps_devices d
    SET last_seen_at = now()
    WHERE d.token_hash = p_token_hash
      AND d.tenant_id = p_tenant_id
      AND (d.last_seen_at IS NULL OR d.last_seen_at < now() - INTERVAL '60 seconds');

    RETURN QUERY
    SELECT t.id, CAST(t.slug AS VARCHAR), CAST(t.name AS VARCHAR), t.database_url, CAST(t.schema_name AS VARCHAR),
           COALESCE(t.is_active, FALSE), COALESCE(t.is_suspended, FALSE), d.vehicle_id
    FROM tenancy.gps_devices d
    INNER JOIN tenancy.tenants t ON t.id = d.tenant_id
    WHERE d.token_hash = p_token_hash
      AND d.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tenancy.sp_gps_device_resolve(UUID, BYTEA) IS
'The tenant and vehicle a GPS device token posts for, or no row; stamps last_seen_at at most once a minute (TRACK-010)';
