CREATE OR REPLACE FUNCTION tenancy.sp_gps_device_issue(p_tenant_id uuid, p_vehicle_id uuid, p_token_hash bytea)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.id = p_tenant_id) THEN
        RAISE EXCEPTION 'tenant.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tenancy.gps_devices AS d (token_hash, tenant_id, vehicle_id)
    VALUES (p_token_hash, p_tenant_id, p_vehicle_id)
    ON CONFLICT ON CONSTRAINT uk_gps_devices_vehicle DO UPDATE
    SET token_hash = EXCLUDED.token_hash, last_seen_at = NULL, created_at = now();
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_gps_device_issue(uuid,uuid,bytea) IS 'Stores the token hash of the vehicle''s GPS device, replacing its previous token; raises tenant.not-found (TRACK-010)';
