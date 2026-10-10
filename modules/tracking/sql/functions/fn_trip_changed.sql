CREATE OR REPLACE FUNCTION tracking.fn_trip_changed(p_tenant_id uuid, p_trip_id uuid, p_type character varying, p_user_id uuid, p_payload jsonb DEFAULT '{}'::jsonb)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
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
        'type',         p_type,
        'subject_id',   COALESCE(p_payload->>'trip_stop_task_id', p_payload->>'trip_stop_id')
    )
    FROM tracking.trips t
    WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_trip_changed(uuid,uuid,character varying,uuid,jsonb) IS 'Records one transition of a trip: a trip_events row of p_type and a trip.changed outbox row { trip_id, route_id, service_date, type, subject_id } — subject_id the task or stop it moved (TRACK-020 D1, TRACK-012)';
