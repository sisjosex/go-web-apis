CREATE OR REPLACE FUNCTION tenancy.sp_gps_device_resolve(p_tenant_id uuid, p_token_hash bytea)
 RETURNS TABLE(tenant_id uuid, slug character varying, name character varying, database_url text, schema_name character varying, is_active boolean, is_suspended boolean, vehicle_id uuid)
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tenancy.sp_gps_device_resolve(uuid,bytea) IS 'The tenant and vehicle a GPS device token posts for, or no row; stamps last_seen_at at most once a minute (TRACK-010)';
