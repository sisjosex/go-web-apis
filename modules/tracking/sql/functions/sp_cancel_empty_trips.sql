CREATE OR REPLACE FUNCTION tracking.sp_cancel_empty_trips(p_tenant_id UUID)
RETURNS TABLE(
    trip_id        UUID,
    route_name     VARCHAR,
    planned_start  TIMESTAMPTZ,
    timezone       VARCHAR,
    driver_user_id UUID
) AS $$
#variable_conflict use_column
DECLARE
    v_row RECORD;
BEGIN
    FOR v_row IN
        -- The route's local today is within a day of UTC's: the range keeps idx_trips_tenant_date_status.
        UPDATE tracking.trips t
        SET status = 'cancelled', is_overridden = TRUE, updated_at = now()
        FROM tracking.routes r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = t.route_id
          AND t.tenant_id = p_tenant_id
          AND t.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
          AND t.status = 'planned'
          AND t.started_at IS NULL
          AND t.service_date = CAST(now() AT TIME ZONE t.timezone AS DATE)
          AND o.tenant_id = p_tenant_id
          AND o.empty_trip_cancel_min > 0
          AND t.planned_start <= now() + make_interval(mins => o.empty_trip_cancel_min)
          AND NOT EXISTS (
              SELECT 1
              FROM tracking.trip_stops ts
              INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
              WHERE ts.trip_id = t.id
                AND k.status <> 'cancelled'
          )
        RETURNING t.id, CAST(r.route_name AS VARCHAR) AS route_name, t.planned_start,
                  CAST(t.timezone AS VARCHAR) AS timezone, t.driver_id
    LOOP
        PERFORM tracking.fn_trip_changed(p_tenant_id, v_row.id, 'cancel', NULL, '{"reason":"no_riders"}');

        trip_id := v_row.id;
        route_name := v_row.route_name;
        planned_start := v_row.planned_start;
        timezone := v_row.timezone;
        driver_user_id := (SELECT d.user_id FROM tracking.drivers d WHERE d.id = v_row.driver_id AND d.tenant_id = p_tenant_id);
        RETURN NEXT;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_cancel_empty_trips(UUID) IS 'Cancels the tenant''s trips of today, not started, with no live task, whose client''s empty_trip_cancel_min lead has been reached (0 never, no client never); each becomes overridden, so the materialiser never revives it, and records a cancel event with payload.reason no_riders; answers each with its route, start, timezone and the driver''s linked user (TRACK-069 D1, D2)';
