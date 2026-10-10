CREATE OR REPLACE FUNCTION tracking.fn_trip_at(p_trip tracking.trips, p_recorded_at timestamp with time zone)
 RETURNS timestamp with time zone
 LANGUAGE sql
 STABLE
AS $function$
    SELECT LEAST(now(), GREATEST(COALESCE(p_recorded_at, now()), COALESCE(p_trip.started_at, now())));
$function$;

COMMENT ON FUNCTION tracking.fn_trip_at(tracking.trips,timestamp with time zone) IS 'The time a transition of the trip is stamped with: p_recorded_at clamped to [started_at, now], now() when NULL (TRACK-011 D2)';
