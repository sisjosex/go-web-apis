CREATE OR REPLACE FUNCTION tenancy.sp_usage_daily_upsert(p_tenant_id uuid, p_day date, p_vehicles_active integer, p_trips_completed integer, p_positions bigint, p_riders_boarded integer)
 RETURNS void
 LANGUAGE sql
AS $function$
    INSERT INTO tenancy.usage_daily AS u (
        tenant_id, day, vehicles_active, trips_completed, positions, riders_boarded, rolled_at
    )
    VALUES (p_tenant_id, p_day, p_vehicles_active, p_trips_completed, p_positions, p_riders_boarded, now())
    ON CONFLICT (tenant_id, day) DO UPDATE SET
        vehicles_active = EXCLUDED.vehicles_active,
        trips_completed = EXCLUDED.trips_completed,
        positions = EXCLUDED.positions,
        riders_boarded = EXCLUDED.riders_boarded,
        rolled_at = EXCLUDED.rolled_at;
$function$;

COMMENT ON FUNCTION tenancy.sp_usage_daily_upsert(uuid,date,integer,integer,bigint,integer) IS 'Writes one tenant''s usage for one day, replacing what a previous pass wrote (TRACK-013 D3)';
