CREATE OR REPLACE FUNCTION tracking.sp_live_fleet(p_tenant_id uuid, p_organization_id uuid DEFAULT NULL::uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(vehicle_id uuid, license_plate character varying, trip_id uuid, route_id uuid, route_name character varying, lat double precision, lng double precision, speed real, heading real, recorded_at timestamp with time zone, organization_kind character varying, driver_name character varying, driver_phone character varying, organization_name character varying, riders_total integer, riders_boarded integer, delay_seconds integer, arrived_at timestamp with time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    -- The organizations whose riders a vehicle's trip must carry; NULL = the whole fleet.
    v_orgs   UUID[] := CASE WHEN p_organization_id IS NOT NULL THEN ARRAY[p_organization_id] END;
    v_riders UUID[];
BEGIN
    -- TRACK-067: an organization user sees only the buses carrying its riders — a supervisor's, its groups'.
    IF p_scope_user_id IS NOT NULL THEN
        v_orgs := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        IF p_organization_id IS NOT NULL THEN
            IF NOT (p_organization_id = ANY(v_orgs)) THEN
                RAISE EXCEPTION 'tracking.organization.scope-denied' USING ERRCODE = 'P0001';
            END IF;
            v_orgs := ARRAY[p_organization_id];
        END IF;
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT
        v.id,
        CAST(v.plate_number AS VARCHAR),
        t.id,
        t.route_id,
        CAST(r.route_name AS VARCHAR),
        ST_Y(lp.location::geometry),
        ST_X(lp.location::geometry),
        lp.speed,
        lp.heading,
        lp.recorded_at,
        CAST(o.kind AS VARCHAR),
        CAST(d.first_name || ' ' || d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(pr.riders AS INT),
        CAST(pr.boarded AS INT),
        -- sp_list_trips' delay: the next pending stop overdue by, 0 while not yet due.
        CASE WHEN ns.planned_at IS NOT NULL THEN CAST(GREATEST(0, EXTRACT(EPOCH FROM (now() - ns.planned_at))) AS INT) END,
        ar.ended_at
    FROM tracking.vehicle_last_position lp
    INNER JOIN tracking.vehicles v ON v.id = lp.vehicle_id
    INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id AND tc.tenant_id = p_tenant_id
    LEFT JOIN LATERAL (
        SELECT tr.id, tr.route_id, tr.driver_id
        FROM tracking.trips tr
        WHERE tr.vehicle_id = v.id
          AND tr.status = 'in_progress'
          AND tr.tenant_id = p_tenant_id
        ORDER BY tr.started_at DESC
        LIMIT 1
    ) t ON true
    LEFT JOIN tracking.routes r ON r.id = t.route_id
    LEFT JOIN tracking.organizations o ON o.id = r.organization_id
    LEFT JOIN tracking.drivers d ON d.id = t.driver_id
    -- TRACK-073: what the bus card shows of a running trip — riders picked up over riders expected
    -- (pickups not cancelled) and its next pending stop; nothing without a trip in progress.
    LEFT JOIN LATERAL (
        SELECT
            COUNT(k.id) FILTER (WHERE k.status <> 'cancelled') AS riders,
            COUNT(k.id) FILTER (WHERE k.status = 'done') AS boarded
        FROM tracking.trip_stops ts
        INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id AND k.kind = 'pickup'
        WHERE ts.trip_id = t.id
    ) pr ON t.id IS NOT NULL
    LEFT JOIN LATERAL (
        SELECT ts.planned_at
        FROM tracking.trip_stops ts
        WHERE ts.trip_id = t.id AND ts.status = 'pending'
        ORDER BY ts.sequence
        LIMIT 1
    ) ns ON t.id IS NOT NULL
    -- "Llegó 07:38": with no trip in progress, the vehicle's last trip of its local today, completed.
    LEFT JOIN LATERAL (
        SELECT tr.ended_at
        FROM tracking.trips tr
        WHERE tr.vehicle_id = v.id
          AND tr.tenant_id = p_tenant_id
          AND tr.status = 'completed'
          AND tr.service_date BETWEEN CURRENT_DATE - 1 AND CURRENT_DATE + 1
          AND tr.service_date = CAST(now() AT TIME ZONE tr.timezone AS DATE)
        ORDER BY tr.ended_at DESC NULLS LAST
        LIMIT 1
    ) ar ON t.id IS NULL
    WHERE v_orgs IS NULL
       OR EXISTS (
            SELECT 1
            FROM tracking.trip_stops ts
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            INNER JOIN tracking.riders rd ON rd.id = k.subject_id
            WHERE ts.trip_id = t.id
              AND k.subject_type = 'passenger'
              AND k.status <> 'cancelled'
              AND rd.organization_id = ANY(v_orgs)
              AND (v_riders IS NULL OR rd.id = ANY(v_riders))
        )
    ORDER BY v.plate_number;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_live_fleet(uuid,uuid,uuid) IS 'Every reporting vehicle of the tenant with its last position, trip in progress, route (id and name) the route client''s kind and name, the driver''s name and phone, riders_total/riders_boarded (pickups expected and done) and delay_seconds of the trip in progress, and arrived_at (its local today''s last completed trip) when none runs (TRACK-073); narrowed to the trips carrying riders of p_organization_id when set (TRACK-025, TRACK-028, TRACK-051), and for an organization user (p_scope_user_id) to its organizations'' — a supervisor''s groups'' — riders, scope-denied for another organization (TRACK-067)';
