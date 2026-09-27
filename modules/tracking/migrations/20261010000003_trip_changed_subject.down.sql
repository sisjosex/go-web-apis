DROP FUNCTION IF EXISTS tracking.sp_notify_event(UUID, VARCHAR, JSONB, BIGINT);

CREATE OR REPLACE FUNCTION tracking.fn_trip_changed(
    p_tenant_id UUID,
    p_trip_id UUID,
    p_type VARCHAR,
    p_user_id UUID,
    p_payload JSONB DEFAULT '{}'::jsonb
)
RETURNS VOID AS $$
BEGIN
    INSERT INTO tracking.trip_events (trip_id, type, payload, created_by)
    SELECT t.id, p_type, COALESCE(p_payload, '{}'::jsonb), p_user_id
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;

    INSERT INTO tracking.outbox (topic, payload)
    SELECT 'trip.changed', jsonb_build_object(
        'trip_id',      t.id,
        'route_id',     t.route_id,
        'service_date', t.service_date,
        'type',         p_type
    )
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.fn_trip_changed(UUID, UUID, VARCHAR, UUID, JSONB) IS
'Records one transition of a trip: a trip_events row of p_type and a trip.changed outbox row { trip_id, route_id, service_date, type } (TRACK-020 D1)';
