CREATE OR REPLACE FUNCTION tracking.sp_rider_live(p_tenant_id uuid, p_rider_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid, p_guardian_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_id uuid, trip_id uuid, route_name character varying, license_plate character varying, stops jsonb, vehicle_position jsonb, pending_stops jsonb, planned_path jsonb, vehicle_type character varying, organization_kind character varying, destination jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_guardian_scope UUID[];
    v_trip UUID;
    v_rider_kind VARCHAR;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;
    IF p_guardian_user_id IS NOT NULL THEN
        v_guardian_scope := tracking.fn_guardian_scope(p_tenant_id, p_guardian_user_id);
    END IF;

    SELECT CAST(o.kind AS VARCHAR) INTO v_rider_kind
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = p_rider_id
      AND o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
      AND (v_guardian_scope IS NULL OR r.id = ANY(v_guardian_scope));
    IF NOT FOUND THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT t.id INTO v_trip
    FROM tracking.trips t
    WHERE t.tenant_id = p_tenant_id
      AND t.status = 'in_progress'
      AND EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.subject_id = p_rider_id
              AND k.status <> 'cancelled')
    ORDER BY t.started_at DESC
    LIMIT 1;

    RETURN QUERY
    SELECT
        p_rider_id,
        t.id,
        CAST(r.route_name AS VARCHAR),
        CAST(v.plate_number AS VARCHAR),
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'stop_name',    sp.name,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry),
                'kind',         k.kind,
                'status',       ts.status,
                'planned_at',   ts.planned_at
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.subject_id = p_rider_id
              AND k.status <> 'cancelled'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        CASE WHEN lp.vehicle_id IS NOT NULL THEN jsonb_build_object(
            'vehicle_id',  lp.vehicle_id,
            'trip_id',     lp.trip_id,
            'lat',         ST_Y(lp.location::geometry),
            'lng',         ST_X(lp.location::geometry),
            'speed',       lp.speed,
            'heading',     lp.heading,
            'accuracy',    lp.accuracy,
            'recorded_at', lp.recorded_at
        ) END,
        COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'trip_stop_id', ts.id,
                'lat',          ST_Y(sp.location::geometry),
                'lng',          ST_X(sp.location::geometry)
            ) ORDER BY ts.sequence)
            FROM tracking.trip_stops ts
            INNER JOIN tracking.stop_places sp ON sp.id = ts.stop_place_id
            WHERE ts.trip_id = t.id
              AND ts.status = 'pending'
              AND sp.location IS NOT NULL
        ), '[]'::jsonb),
        tracking.fn_trip_planned_path(t.route_version_id),
        CAST(v.vehicle_type AS VARCHAR),
        CASE WHEN t.id IS NOT NULL THEN COALESCE(CAST(o.kind AS VARCHAR), v_rider_kind) END,
        tracking.fn_trip_destination(p_tenant_id, t.id)
    FROM (SELECT v_trip AS id) cur
    LEFT JOIN tracking.trips t ON t.id = cur.id
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.vehicles v ON v.id = t.vehicle_id
    LEFT JOIN tracking.vehicle_last_position lp ON lp.vehicle_id = t.vehicle_id;

    PERFORM tracking.fn_record_rider_access(
        p_tenant_id, COALESCE(p_guardian_user_id, p_scope_user_id),
        CASE WHEN p_guardian_user_id IS NOT NULL THEN 'portal' ELSE 'organization' END, 'live', ARRAY[p_rider_id]);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_live(uuid,uuid,uuid,uuid) IS 'The rider''s trip in progress: its stops for the rider with coordinates, the vehicle''s last position, the trip''s pending stops for the ETA, the planned line, the vehicle type, the client''s kind and the destination; trip_id NULL when none; raises rider.not-found out of scope; a portal or organization read is logged in rider_access_log (MOBILE-010, MOBILE-023, TRACK-063)';
