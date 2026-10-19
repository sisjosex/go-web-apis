CREATE OR REPLACE FUNCTION tenancy.sp_list_usage_daily(p_from date, p_to date, p_tenant_id uuid)
 RETURNS TABLE(tenant_id uuid, slug character varying, name character varying, days jsonb, vehicles_active_max integer, vehicles_active_avg numeric, trips_completed integer)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        t.id,
        CAST(t.slug AS VARCHAR),
        CAST(t.name AS VARCHAR),
        jsonb_agg(jsonb_build_object(
            'day', u.day, 'vehicles_active', u.vehicles_active, 'trips_completed', u.trips_completed,
            'positions', u.positions, 'riders_boarded', u.riders_boarded) ORDER BY u.day),
        CAST(MAX(u.vehicles_active) AS INT),
        ROUND(AVG(u.vehicles_active), 1),
        CAST(SUM(u.trips_completed) AS INT)
    FROM tenancy.usage_daily u
    INNER JOIN tenancy.tenants t ON t.id = u.tenant_id
    WHERE u.day BETWEEN p_from AND p_to
      AND (p_tenant_id IS NULL OR u.tenant_id = p_tenant_id)
    GROUP BY t.id, t.slug, t.name
    ORDER BY t.slug;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_usage_daily(date,date,uuid) IS 'Each tenant''s usage rows between two days with the range''s peak and mean active vehicles and its trips; one tenant when p_tenant_id is set (TRACK-013 D3)';
